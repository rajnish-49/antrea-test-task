# Packet Capture Controller

Watches pods and runs tcpdump when annotated with `tcpdump.antrea.io`.

## Setup

```
make cluster
make docker-build
make kind-load
make deploy
```

## Usage

Start capture:
```
kubectl annotate pod test-pod tcpdump.antrea.io="5"
```

Stop capture:
```
kubectl annotate pod test-pod tcpdump.antrea.io-
```

## Collect outputs

```
make collect-outputs
```

## Cleanup

```
make cluster-delete
```
