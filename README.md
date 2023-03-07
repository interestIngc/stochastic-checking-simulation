# Stochastic checking simulation

## Command to start the main server (must be called before starting nodes)

```
go run simulation/mainserver/*.go --n @{N} --times @{Times} --log_file @{LogFile}
```

### Where
@{N} - number of processes in the system (excluding the main server)  
@{Times} - number of transactions for each process to broadcast, defaults to 5  
@{LogFile} - path to the file where to save logs produced by the mainserver

### Example command

```
go run simulation/mainserver/*.go --n 2 --times 3 --log_file mainserver.txt
```

## Command to start a node:

```
go run simulation/node/main.go --input_file @{InputFile} -i @{I} --log_file @{LogFile}
```

### Where
@{I} - index of the current process in the system from 0 to @{N} - 1  
@{LogFile} - path to the file where to save logs produced by the process  
@{InputFile} - path to the input file in json format.

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
