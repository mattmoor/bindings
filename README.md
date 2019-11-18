# Knative-style Bindings

This repository contains a collection of Knative-style Bindings for accessing
various services.  Each of the bindings contained in this repository generally
has two key parts:

1. A Binding CRD that will augment the runtime contract of the Binding subject
   in some way.

2. A small client library for consuming the runtime conract alteration to
   bootstrap and API client for the service being bound.


## `GithubBinding`

The `GithubBinding` is intended to facilitate the consumption of the GitHub API.
It has the following form:

```yaml
apiVersion: bindings.mattmoor.dev/v1alpha1
kind: GithubBinding
metadata:
  name: foo-binding
spec:
  subject:
    apiVersion: apps/v1
    kind: Deployment
    # Either name or selector may be specified.
    selector:
      matchLabels:
        foo: bar

  secret:
    name: github-secret
```

The referenced secret should have a key named `accessToken` with the Github
access token to be used with the Github API.  It and any other keys are made
available under `/var/bindings/github/` (this is the runtime contract of the
`GithubBinding`).

There is a helper library available to aid in the consumption of this runtime
contract, which returns a `github.com/google/go-github/github.Client`:

```go

import "github.com/mattmoor/bindings/pkg/github"


// Instantiate a Client from the access token made available by
// the GithubBinding.
client, err := github.New(ctx)
...

```


## `SlackBinding`

The `SlackBinding` is intended to facilitate the consumption of the Slack API.
It has the following form:

```yaml
apiVersion: bindings.mattmoor.dev/v1alpha1
kind: SlackBinding
metadata:
  name: foo-binding
spec:
  subject:
    apiVersion: apps/v1
    kind: Deployment
    # Either name or selector may be specified.
    selector:
      matchLabels:
        foo: bar

  secret:
    name: slack-secret
```

The referenced secret should have a key named `token` with the Slack
token to be used with the Slack API.  It and any other keys are made
available under `/var/bindings/slack/` (this is the runtime contract of the
`SlackBinding`).

There is a helper library available to aid in the consumption of this runtime
contract, which returns a `github.com/nlopes/slack.Client`:

```go

import "github.com/mattmoor/bindings/pkg/slack"


// Instantiate a Client from the token made available by
// the SlackBinding.
client, err := slack.New(ctx)
...

```
