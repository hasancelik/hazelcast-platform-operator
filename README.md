
<img src="https://hazelcast.com/files/brand-images/logo/hazelcast-logo.png">
<br />

# Hazelcast Platform Operator #
[![Distribution Tests](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/k8s-dist-tests.yaml/badge.svg)](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/k8s-dist-tests.yaml)

Easily deploy Hazelcast clusters and Management Center into Kubernetes/OpenShift environments and manage their lifecycles.

Hazelcast Platform Operator is based on the [Operator SDK](https://github.com/operator-framework/operator-sdk) to follow best practices.

Here is a short video to deploy a simple Hazelcast Platform cluster and Management Center via Hazelcast Platform Operator:

[Deploy a Cluster With the Hazelcast Platform Operator for Kubernetes](https://www.youtube.com/watch?v=4cK5I74nmr4)

## Table of Contents

* [Documentation](#documentation)
* [Features](#features)
* [Contribute](#contribute)
* [License](#license)

## Documentation

1. [Get started](https://docs.hazelcast.com/operator/latest/get-started) with the Operator
2. [Connect the cluster from outside Kubernetes](https://docs.hazelcast.com/tutorials/hazelcast-platform-operator-expose-externally)
3. [Restore a Cluster from Cloud Storage with Hazelcast Platform Operator](https://docs.hazelcast.com/tutorials/hazelcast-platform-operator-external-backup-restore)
4. [Replicate Data between Two Hazelcast Clusters with Hazelcast Platform Operator](https://docs.hazelcast.com/tutorials/hazelcast-platform-operator-wan-replication)
5. [Configure MongoDB Atlas as an External Data Store for the Cluster with Hazelcast Platform Operator](https://docs.hazelcast.com/tutorials/hazelcast-platform-operator-map-store-mongodb-atlas)

## Features

Hazelcast Platform Operator supports the features below:

* Custom resource for Hazelcast Platform (Enterprise) and Management Center
* Observe status of Hazelcast clusters and Management Center instances
* High Availability Mode configuration to create clusters that are resilient to node and zone failures
* Support for TLS connections between members using self-signed certificates
* Support LDAP Security Provider for Management Center CR
* Scale up and down Hazelcast clusters
* Expose Hazelcast cluster to external
  clients ([Smart & Unisocket](https://docs.hazelcast.com/hazelcast/latest/clients/java#java-client-operation-modes))
* Backup Hazelcast persistence data to cloud storage with the possibility of scheduling it and restoring the data accordingly
* WAN Replication feature when you need to synchronize multiple Hazelcast clusters, which are connected by WANs
* Full and Delta WAN Sync support
* Tiered Storage support for Hazelcast and Map CRs
* CP Subsystem configuration support for Hazelcast CR
* User Code Deployment feature, which allows you to deploy custom and domain classes from cloud storages and URLs to Hazelcast members
* The User Code Namespaces feature allows you to deploy custom and domain classes at runtime without a members restart
* Support the configuration of advanced networking options
* Support Multi-namespace configuration
* ExecutorService and EntryProcessor support
* Support several data structures like Map, Topic, MultiMap, ReplicatedMap, Queue and Cache which can be created dynamically via specific Custom Resources
* MapStore, Near Cache and off-heap memory (HD memory and native memory) support for the Map CR
* Native Memory support for the Cache CR
* Support Jet configuration and Jet Job submission using the JetJob CR
* Support for exporting the snapshots of JetJob CRs using JetJobSnapshot CR
* Support for custom configurations using ConfigMap

For Hazelcast Platform Enterprise, you can request a trial license key from [here](https://trialrequest.hazelcast.com).

## Contribute

Before you contribute to the Hazelcast Platform Operator, please read the following:

* [Contributing to Hazelcast Platform Operator](CONTRIBUTING.md)
* [Developing and testing Hazelcast Platform Operator](DEVELOPER.md)
* [Hazelcast Platform Operator Architecture](ARCHITECTURE_OVERVIEW.md)

## License

Please see the [LICENSE](LICENSE) file.

# Hazelcast Platform Operator Agent #

<img align="right" src="https://hazelcast.com/brand-assets/files/hazelcast-stacked-flat-sm.png">

Platform Operator Agent enables users to utilize Hazelcast Platform's features easily in Kubernetes environments.
The agent is implemented using [Go CDK](https://gocloud.dev/) for cloud providers support which allows the agent to become mostly cloud provider agnostic. Supported providers are: AWS, GCP and Azure.

The agent is used by Hazelcast Platform Operator for supporting multiple features. The features are:

- [User Code Deployment](#user-code-deployment)
- [Restore](#restore)
- [Backup](#backup)

## User Code Deployment

There are two commands for user code deployment: `bucket-jar-download` and `url-file-download`

### User Code from Buckets

Agent downloads `jar` files from a specified bucket and puts it under destined path. Learn more about `bucket-jar-download` command using the `--help` argument.

### User Code from URLs

Agent downloads files from a specified URLs and puts them under destined path. Learn more about `url-file-download` command using the `--help` argument.

## Restore

Agent restores backup files stored as `.tar.gz` archives from specified bucket and puts the files under destined path. Learn more about `restore` command using the `--help` argument.

## Backup

Backup command starts an HTTP server for Backup related tasks. Learn more about `backup` command using the `--help` argument. It exposes the following endpoints:

- `POST /upload`: Agent starts an asynchronous backup process. It uploads the latest Hazelcast backup into specified bucket, arhiving the folder in the process. Returns an id of the backup process.
- `GET /upload/{id}`: Returns the status of the backup.
- `POST /upload/{id}/cancel`: Cancels the backup process.
- `DELETE /upload/{id}`: Deletes the backup process status.
- `GET /health`: Returns success if application is running.

## License

Please see the [LICENSE](../../hazelcast-platform-operator-agent/LICENSE) file.
