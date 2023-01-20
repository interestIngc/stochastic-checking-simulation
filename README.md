# Stochastic checking simulation

## Command to start the main server (must be called before starting nodes)

```
go run mainserver/server.go mainserver/main.go --mainserver @{MainServerAddress} --n @{N} --f @{F} --w @{W} --u @{U} --node_id_size @{NodeIdSize} --number_of_bins @{NumberOfBins}
```

### Where
@{MainServerAddress} - address of the main server, e.g. 127.0.0.1:8080  
@{N} - the number of processes in the system (excluding the main server)  
@{F} - the max number of faulty processes in the system  
@{W} - size of the witness set  
@{U} - witnesses threshold to accept a transaction  
@{NodeIdSize} - node id size, default is 256 (optional)  
@{NumberOfBins} - number of bins in history hash, default is 32 (optional)

### Example command

```
go run mainserver/server.go mainserver/main.go --mainserver 127.0.0.1:8080 --n 2 --f 0 --w 2 --u 2
```

## Command to start a node:

```
go run simulation/*.go --nodes @{Nodes} --mainserver @{MainServerAddress} --protocol @{Protocol} --address @{NodeAddress} --f @{F} --w @{W} --u @{U} --node_id_size @{NodeIdSize} --number_of_bins @{NumberOfBins}
```

### Where
@{Nodes} is a string representing bindings host:port separated by comma, e.g. 127.0.0.1:8081,127.0.0.1:8082  
@{MainServerAddress} is address of the main server, e.g. 127.0.0.1:8080  
@{Protocol} is a protocol to run, either accountability or broadcast  
@{NodeAddress} is current node's address, e.g. 127.0.0.1:8081  
@{F} - the max number of faulty processes in the system  
@{W} - size of the witness set  
@{U} - witnesses threshold to accept a transaction  
@{NodeIdSize} - node id size, default is 256 (optional)  
@{NumberOfBins} - number of bins in history hash, default is 32 (optional)

### Example command

```
go run simulation/*.go --nodes 127.0.0.1:8081,127.0.0.1:8082 --mainserver 127.0.0.1:8080 --protocol accountability --address 127.0.0.1:8082 --f 0 --w 2 --u 2
```
