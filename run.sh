#!/bin/zsh

n=2

go run simulation/mainserver/*.go --log_file outputs/mainserver.txt --ip 127.0.0.1 --port 8080 --n "$n" --local_run true &

for (( i = 0; i < $n; i++ ))
do
  go run simulation/node/main.go --input_file input.json --base_ip 127.0.0.1 --port 8080 --i $i \
  --log_file "outputs/process${i}.txt" --local_run true &
done
