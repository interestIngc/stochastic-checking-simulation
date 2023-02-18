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
go run simulation/node/main.go --file @{PathToInputFile}
```

### Where
@{PathToInputFile} is an absolute path to the input file containing configuration of the current node  

Content of the input file should be as follows:

1. Each line of the file must represent one parameter, i.e.
```
parameter @{ParameterValue}
```
2. Parameters might appear in any order, some of them, which are marked as optional, 
might be missing in case they are not required for the selected protocol to run.
3. Description of the parameters  
    * Mandatory:
        * nodes - string representing bindings host:port separated by comma, e.g. 127.0.0.1:8081,127.0.0.1:8082
        * current_node - current node's address, e.g. 127.0.0.1:8081
        * main_server - address of the main server, e.g. 127.0.0.1:8080
        * protocol - a protocol to run, one of:
            * reliable_accountability - byzantine reliable broadcast protocol based on stochastic accountability
            * consistent_accountability - byzantine consistent broadcast protocol based on stochastic accountability
            * bracha - Bracha protocol, a classical implementation of byzantine reliable broadcast
            * scalable - scalable byzantine reliable broadcast protocol
    * Optional:
        * f - max number of faulty processes in the system
        * w - size of the own witness set W within accountability protocols
        * v - size of the pot witness set V within accountability protocols
        * wr - own witness set radius within accountability protocols
        * vr - pot witness set radius within accountability protocols
        * u - witnesses threshold to accept a transaction within accountability protocols
        * recovery_timeout - timeout to wait (ns) for the process after initialising a message before 
switching to the recovery protocol in case value was not delivered during the given amount of time. 
Used in the reliable accountability protocol
        * node_id_size - node id size within accountability protocols
        * number_of_bins - number of bins in history hash within accountability protocols
        * g_size - gossip sample size within scalable reliable broadcast
        * e_size - echo sample size within scalable reliable broadcast
        * e_threshold - echo threshold within scalable reliable broadcast
        * r_size - ready sample size within scalable reliable broadcast
        * r_threshold - ready threshold within scalable reliable broadcast
        * d_size - delivery sample size within scalable reliable broadcast
        * d_threshold - delivery threshold within scalable reliable broadcast

### Example of the input file

```
nodes 127.0.0.1:8081,127.0.0.1:8082
current_node 127.0.0.1:8081
main_server 127.0.0.1:8080
protocol reliable_accountability
f 0
w 2
v 2
wr 1900.0
vr 1910.0
u 2
recovery_timeout 1000000
node_id_size 256
number_of_bins 32
```