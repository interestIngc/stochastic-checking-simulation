# Stochastic checking simulation

## Command to start the main server (must be called before starting nodes)

```
go run simulation/mainserver/server.go simulation/mainserver/main.go --mainserver @{MainServerAddress} --n @{N}
```

### Where
@{MainServerAddress} - address of the main server, e.g. 127.0.0.1:8080  
@{N} - the number of processes in the system (excluding the main server)  

### Example command

```
go run simulation/mainserver/server.go simulation/mainserver/main.go --mainserver 127.0.0.1:8080 --n 2
```

## Command to start a node:

```
go run simulation/node/main.go --nodes @{Nodes} --mainserver @{MainServerAddress} --protocol @{Protocol} \
--address @{NodeAddress} --f @{F} --w @{W} --v @{V} --u @{U} \
--node_id_size @{NodeIdSize} --number_of_bins @{NumberOfBins}
```

### Where
@{Nodes} - string representing bindings host:port separated by comma, e.g. 127.0.0.1:8081,127.0.0.1:8082   
@{MainServerAddress} - address of the main server, e.g. 127.0.0.1:8080  
@{Protocol} - a protocol to run, one of: reliable_accountability, consistent_accountability, bracha, 
default reliable_accountability (optional)  
@{NodeAddress} - current node's address, e.g. 127.0.0.1:8081  
@{F} - max number of faulty processes in the system  
@{W} - size of the own witness set W  
@{V} - size of the pot witness set V  
@{U} - witnesses threshold to accept a transaction  
@{NodeIdSize} - node id size, default 256 (optional)  
@{NumberOfBins} - number of bins in history hash, default 32 (optional)  

### Example command

```
go run simulation/node/main.go --nodes 127.0.0.1:8081,127.0.0.1:8082 --mainserver 127.0.0.1:8080 \
--protocol reliable_accountability --address 127.0.0.1:8081 \
--f 0 --w 2 --v 2 --u 2
```
