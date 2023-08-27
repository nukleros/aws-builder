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

```bash
./bin/aws-builder create rds sample/rds-config.yaml
```

TODO: library usage example

