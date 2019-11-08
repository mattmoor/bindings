/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"log"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	client, err := cloudevents.NewDefaultClient()
	if err != nil {
		panic(err)
	}

	r := Receiver{client: client}
	if err := envconfig.Process("", &r); err != nil {
		panic(err)
	}

	if err := client.StartReceiver(context.Background(), r.Receive); err != nil {
		log.Fatal(err)
	}
}

type Receiver struct {
	client cloudevents.Client

	Target string `envconfig:"SINK" required:"true"`
}

func (r *Receiver) Receive(event cloudevents.Event) {
	ctx := cloudevents.ContextWithTarget(context.Background(), r.Target)
	if _, _, err := r.client.Send(ctx, event); err != nil {
		fmt.Println(err)
	}
}
