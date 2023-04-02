# Stochastic checking simulation

## Command to start the main server (must be called before starting nodes)

```
go run simulation/mainserver/*.go --n @{N} --log_file @{LogFile} --ip @{Ip} --port @{Port}
```

### Where
@{N} - number of processes in the system (excluding the main server)  
@{LogFile} - path to the file where to save logs produced by the main server  
@{Ip} - ip address of the main server, defaults to 10.0.0.1  
@{Port} - Port on which the main server should be started, defaults to 5001  

### Example command

```
go run simulation/mainserver/*.go --n 2 --log_file mainserver.txt --ip 127.0.0.1 --port 8080
```

## Command to start a node:

```
go run simulation/node/main.go --input_file @{InputFile} --log_file @{LogFile} --i @{I} \
--transactions @{Transactions} --transaction_init_timeout_ns @{TransactionInitTimeoutNs} \
--base_ip @{BaseIp} --port @{Port}
```

### Where
@{LogFile} - path to the file where to save logs produced by the process  
@{InputFile} - path to the input file in json format  
@{I} - index of the current process in the system from 0 to @{N} - 1  
@{Transactions} - number of transactions for the process to broadcast, defaults to 5  
@{TransactionInitTimeoutNs} - timeout the process should wait before initialising a new transaction, defaults to 10000000  
@{BaseIp} - address of the main server, defaults to 10.0.0.1. 
Ip addresses for nodes are assigned by incrementing base_ip n times  
@{Port} - port on which the node should be started, defaults to 5001  

#### Description of the input file

Input file should be presented in the following json format:
```
{
  "protocol": @{Protocol}
  "parameters": @{Parameters}
}
```

1. @{Protocol} - a protocol to run, one of:
    * reliable_accountability - byzantine reliable broadcast protocol based on stochastic accountability
    * consistent_accountability - byzantine consistent broadcast protocol based on stochastic accountability
    * bracha - Bracha protocol, a classical implementation of byzantine reliable broadcast
    * scalable - scalable byzantine reliable broadcast protocol
    
2. @{Parameters} - a json representing parameters required for the selected protocol to run, which might be listed in any order. 
Description of the parameters: 
    * General
        * n - number of processes in the system
        * f - max number of faulty processes in the system
    * Stochastic accountability
        * w - minimal size of the own witness set W
        * v - minimal size of the pot witness set V
        * wr - own witness set radius
        * vr - pot witness set radius
        * u - witnesses threshold to accept a transaction
        * recovery_timeout - timeout to wait (ns) for the process after initialising a message before 
switching to the recovery protocol in case value was not delivered during the given amount of time. 
Used only in the reliable accountability protocol
        * node_id_size - node id size
        * number_of_bins - number of bins in history hash
    * Scalable reliable broadcast
        * g_size - gossip sample size
        * e_size - echo sample size
        * e_threshold - echo threshold
        * r_size - ready sample size
        * r_threshold - ready threshold
        * d_size - delivery sample size
        * d_threshold - delivery threshold

### Example of the input file

```
{
  "protocol": "reliable_accountability",
  "parameters": {
    "n": 2,
    "f": 0,
    "w": 2,
    "v": 2,
    "wr": 1900.0,
    "vr": 1910.0,
    "u": 2,
    "recovery_timeout": 1000000,
    "node_id_size": 256,
    "number_of_bins": 32
  }
}
```

### Example command

```
go run simulation/node/main.go --input_file input.json --log_file process0.txt --i 0 \
--base_ip 127.0.0.1 --port 8080 --transactions 10 --transaction_init_timeout_ns 1000000
```
