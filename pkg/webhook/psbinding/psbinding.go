/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless requ ired by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package psbinding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/markbates/inflect"
	"github.com/mattmoor/bindings/pkg/apis/bindings/v1alpha1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1beta1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

// reconciler implements the AdmissionController for resources
type reconciler struct {
	name string
	path string

	client       kubernetes.Interface
	mwhlister    admissionlisters.MutatingWebhookConfigurationLister
	secretlister corelisters.SecretLister
	listall      ListAll

	WithContext BindableContext

	// lock protects access to exact and inexact
	lock    sync.RWMutex
	exact   map[string]Bindable
	inexact map[string][]pair

	secretName string
}

type pair struct {
	selector labels.Selector
	sb       Bindable
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (ac *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := ac.secretlister.Secrets(system.Namespace()).Get(ac.secretName)
	if err != nil {
		logger.Errorf("Error fetching secret: %v", err)
		return err
	}
	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", ac.secretName, certresources.CACert)
	}

	// Reconcile the webhook configuration.
	return ac.reconcileMutatingWebhook(ctx, caCert)
}

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}

// Admit implements AdmissionController
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	logger := logging.FromContext(ctx)
	switch request.Operation {
	case admissionv1beta1.Create, admissionv1beta1.Update:
	default:
		logger.Infof("Unhandled webhook operation, letting it through %v", request.Operation)
		return &admissionv1beta1.AdmissionResponse{Allowed: true}
	}

	orig := &v1alpha1.PodSpeccable{}
	decoder := json.NewDecoder(bytes.NewBuffer(request.Object.Raw))
	if err := decoder.Decode(&orig); err != nil {
		return webhook.MakeErrorStatus("unable to decode object: %v", err)
	}

	// Look up the Bindable for this resource.
	fb := func() Bindable {
		ac.lock.RLock()
		defer ac.lock.RUnlock()

		// Always try to find an exact match first.
		key := fmt.Sprintf("%s/%s/%s/%s", request.Kind.Group, request.Kind.Kind, request.Namespace, orig.Name)
		if sb, ok := ac.exact[key]; ok {
			return sb
		}

		// The key for inexact omits name.
		key = fmt.Sprintf("%s/%s/%s", request.Kind.Group, request.Kind.Kind, request.Namespace)
		pl, ok := ac.inexact[key]
		if !ok {
			logger.Infof("No entry for key %q", key)
			return nil
		}

		// Iterate over the the inexact matches for this GVK + namespace and return the first
		// Bindable that matches our selector.
		ls := labels.Set(orig.Labels)
		for _, p := range pl {
			if p.selector.Matches(ls) {
				return p.sb
			}
		}
		logger.Infof("No match found for key %q with %d pairs", key, len(pl))
		// If none match, then return nil
		return nil
	}()
	if fb == nil {
		// This doesn't apply!
		return &admissionv1beta1.AdmissionResponse{Allowed: true}
	}

	ctx = ac.WithContext(ctx, fb)

	// Mutate a copy according to the deletion state of the Bindable.
	delta := orig.DeepCopy()
	if fb.GetDeletionTimestamp() != nil {
		fb.Undo(ctx, delta)
	} else {
		fb.Do(ctx, delta)
	}

	// Synthesize a patch from the changes and return it in our AdmissionResponse
	patches, err := duck.CreatePatch(orig, delta)
	if err != nil {
		return webhook.MakeErrorStatus("unable to create patch with binding: %v", err)
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return webhook.MakeErrorStatus("mutation failed: %v", err)
	}
	logger.Infof("Kind: %q PatchBytes: %v", request.Kind, string(patchBytes))

	return &admissionv1beta1.AdmissionResponse{
		Patch:   patchBytes,
		Allowed: true,
		PatchType: func() *admissionv1beta1.PatchType {
			pt := admissionv1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (ac *reconciler) reconcileMutatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)

	fbs, err := ac.listall()
	if err != nil {
		return err
	}

	// Build a deduplicated list of all of the GVKs we see
	gks := map[schema.GroupKind]sets.String{
		appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind():  sets.NewString("v1"),
		appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind(): sets.NewString("v1"),
		appsv1.SchemeGroupVersion.WithKind("DaemonSet").GroupKind():   sets.NewString("v1"),
		batchv1.SchemeGroupVersion.WithKind("Job").GroupKind():        sets.NewString("v1"),
	}

	exact := make(map[string]Bindable, len(fbs))
	inexact := make(map[string][]pair, len(fbs))

	for _, fb := range fbs {
		ref := fb.GetSubject()
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return err
		}
		gk := schema.GroupKind{
			Group: gv.Group,
			Kind:  ref.Kind,
		}
		set := gks[gk]
		if set == nil {
			set = sets.NewString()
		}
		set.Insert(gv.Version)
		gks[gk] = set

		if ref.Name != "" {
			exact[fmt.Sprintf("%s/%s/%s/%s", gk.Group, gk.Kind, ref.Namespace, ref.Name)] = fb
		} else {
			selector, err := metav1.LabelSelectorAsSelector(ref.Selector)
			if err != nil {
				return err
			}
			key := fmt.Sprintf("%s/%s/%s", gk.Group, gk.Kind, ref.Namespace)
			pl := inexact[key]
			pl = append(pl, pair{
				selector: selector,
				sb:       fb,
			})
			inexact[key] = pl
			logger.Infof("Adding entry for %q", key)
		}
	}

	// Update our indices
	func() {
		ac.lock.Lock()
		defer ac.lock.Unlock()
		ac.exact = exact
		ac.inexact = inexact
	}()

	var rules []admissionregistrationv1beta1.RuleWithOperations
	for gk, versions := range gks {
		plural := strings.ToLower(inflect.Pluralize(gk.Kind))

		rules = append(rules, admissionregistrationv1beta1.RuleWithOperations{
			Operations: []admissionregistrationv1beta1.OperationType{
				admissionregistrationv1beta1.Create,
				admissionregistrationv1beta1.Update,
			},
			Rule: admissionregistrationv1beta1.Rule{
				APIGroups:   []string{gk.Group},
				APIVersions: versions.List(),
				Resources:   []string{plural + "/*"},
			},
		})
	}

	// Sort the rules by Group, Version, Kind so that things are deterministically ordered.
	sort.Slice(rules, func(i, j int) bool {
		lhs, rhs := rules[i], rules[j]
		if lhs.APIGroups[0] != rhs.APIGroups[0] {
			return lhs.APIGroups[0] < rhs.APIGroups[0]
		}
		if lhs.APIVersions[0] != rhs.APIVersions[0] {
			return lhs.APIVersions[0] < rhs.APIVersions[0]
		}
		return lhs.Resources[0] < rhs.Resources[0]
	})

	configuredWebhook, err := ac.mwhlister.Get(ac.name)
	if err != nil {
		return fmt.Errorf("error retrieving webhook: %v", err)
	}
	webhook := configuredWebhook.DeepCopy()

	// Clear out any previous (bad) OwnerReferences.
	// See: https://github.com/knative/serving/issues/5845
	webhook.OwnerReferences = nil

	// Use the "Equivalent" match policy so that we don't need to enumerate versions for same-types.
	// This is only supported by 1.15+ clusters.
	matchPolicy := admissionregistrationv1beta1.Equivalent

	// We need to specifically exclude our deployment(s) from consideration, but this provides a way
	// of excluding other things as well.
	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			// TODO(mattmoor): Hoist constant and document
			Key:      "bindings.mattmoor.dev/exclude",
			Operator: metav1.LabelSelectorOpNotIn,
			Values:   []string{"true"},
		}},
	}

	for i, wh := range webhook.Webhooks {
		if wh.Name != webhook.Name {
			continue
		}
		webhook.Webhooks[i].MatchPolicy = &matchPolicy
		webhook.Webhooks[i].Rules = rules
		webhook.Webhooks[i].NamespaceSelector = &selector
		webhook.Webhooks[i].ObjectSelector = &selector // 1.15+ only
		webhook.Webhooks[i].ClientConfig.CABundle = caCert
		if webhook.Webhooks[i].ClientConfig.Service == nil {
			return fmt.Errorf("missing service reference for webhook: %s", wh.Name)
		}
		webhook.Webhooks[i].ClientConfig.Service.Path = ptr.String(ac.Path())
	}

	if ok, err := kmp.SafeEqual(configuredWebhook, webhook); err != nil {
		return fmt.Errorf("error diffing webhooks: %v", err)
	} else if !ok {
		logger.Info("Updating webhook")
		mwhclient := ac.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations()
		if _, err := mwhclient.Update(webhook); err != nil {
			return fmt.Errorf("failed to update webhook: %v", err)
		}
	} else {
		logger.Info("Webhook is valid")
	}
	return nil
}
