#!/bin/zsh

n=$1

go run ../simulation/mainserver/*.go --log_file=../outputs/mainserver.txt --base_ip=127.0.0.1 --base_port=8080 --nodes=1 --n="$n" --topo="star" &

for (( i = 0; i < $n; i++ ))
do
  go run ../simulation/node/*.go --input_file=../input.json --base_ip=127.0.0.1 --base_port=8080 --nodes=1 --i="$i" \
  --log_file="../outputs/process${i}.txt" --keys_dir="../keys" --transactions="$3" --transaction_init_timeout_ns="$4" --topo="star" --mixing_time=100 &
done

sleep "$2"

for (( i = 0; i <= $n; i++ ))
do
  kill -9 $(lsof -t -i :$(( 8080 + i )))
done
