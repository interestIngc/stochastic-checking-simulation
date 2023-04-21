#!/bin/zsh

n=70

go run local/distributed/mainserver/*.go --log_file outputs/mainserver.txt --ip 127.0.0.1 --port 8080 --n "$n" &

for (( i = 0; i < $n; i++ ))
do
  go run local/distributed/node/main.go --input_file input.json --base_ip 127.0.0.1 --port 8080 --i $i \
  --log_file "outputs/process${i}.txt" &
done
