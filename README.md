# Stochastic checking simulation

## Command to start the main server (must be called before starting nodes)

```
go run mainserver/server.go mainserver/main.go --mainserver @{MainServerAddress}
```

### Where
@{MainServerAddress} is address of the main server, e.g. 127.0.0.1:8080

### Example command

```
go run mainserver/server.go mainserver/main.go --mainserver 127.0.0.1:8080
```

## Command to start a node:

```
go run simulation/*.go --nodes @{Nodes} --mainserver @{MainServerAddress} --protocol @{Protocol} --address @{NodeAddress}
```

### Where
@{Nodes} is a string representing bindings host:port separated by comma, e.g. 127.0.0.1:8081,127.0.0.1:8082  
@{MainServerAddress} is address of the main server, e.g. 127.0.0.1:8080  
@{Protocol} is a protocol to run, either accountability or broadcast  
@{NodeAddress} is current node's address, e.g. 127.0.0.1:8081

### Example command

```
go run simulation/*.go --nodes 127.0.0.1:8081,127.0.0.1:8082 --mainserver 127.0.0.1:8080 --protocol accountability --address 127.0.0.1:8082
```
