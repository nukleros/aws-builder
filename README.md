# aws-builder

Manage AWS resource stacks for different managed services.

A go library and CLI for managing AWS managed services.

Currently supported services:
* Elastic Kubernetes Service (EKS)
* Relational Database Service (RDS)
* Simple Storage Service (S3)

## Quickstart

Build the CLI:

```bash
make build
```

Create an EKS cluster resource stack:

Edit the `sample/eks-config.yaml` file to your requirements.

```bash
./bin/aws-builder create eks sample/eks-config.yaml
```

Create an RDS instance resource stack:

Edit the `sample/rds-config.yaml` file to match your environment.

```bash
./bin/aws-builder create rds sample/rds-config.yaml
```

Create an S3  resource stack:

Edit the `sample s3-config.yaml` file to match your requirements.

```bash
./bin/aws-builder create s3 sample/s3-config.yaml
```

## Library

For examples of how to use the library to manage AWS resources in a go program,
see the [create](cmd/aws-builder/cmd/create.go) and
[delete](cmd/aws-builder/cmd/delete.go) command source code.

## Tagging Resource Stacks

For each distinct resource stack, apply unique tags.

The config for each resource stack supports adding custom tags.  It's important
to apply unique tags to your resource stacks to support idempotent creation.
Using this project, the tags on resources will be checked to see if they belong
to the resource stack being created.


