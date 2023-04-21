#!/bin/zsh

n=70
pids=()

go run localsimulation/mainserver/*.go --log_file outputs/mainserver.txt --ip 127.0.0.1 --port 8080 --n "$n" & pids+=($!)

for (( i = 0; i < $n; i++ ))
do
  go run localsimulation/node/main.go --input_file input.json --base_ip 127.0.0.1 --port 8080 --i $i \
  --log_file "outputs/process${i}.txt" & pids+=($!)
done

echo "${pids[@]}"
