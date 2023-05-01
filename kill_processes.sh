#!/bin/zsh

n=3

for (( i = 0; i < $n; i++ ))
do
  kill -9 $(lsof -t -i :$(( 8080 + i )))
done
