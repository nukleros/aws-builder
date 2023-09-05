# aws-builder

Manage AWS resource stacks for different managed services.

A go library and CLI for managing AWS managed services.

Currently supported services:
* Relational Database Service

## Quickstart

Build the CLI:

```bash
make build
```

Create an RDS instance:

Edit the `sample/rds-config.yaml` file to match your environment.

```bash
./bin/aws-builder create rds sample/rds-config.yaml
```

## Library

For examples of how to use the library to manage AWS resources in a go program,
see the [create](cmd/aws-builder/cmd/create.go) and
[delete](cmd/aws-builder/cmd/delete.go) command source code.

