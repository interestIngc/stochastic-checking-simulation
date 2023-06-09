#!/usr/bin/python

from time import time, sleep
from signal import SIGINT
from subprocess import call
import sys

from mininet.topo import Topo
from mininet.net import Mininet
from mininet.util import dumpNodeConnections
from mininet.log import setLogLevel, info
from mininet.link import TCULink
from mininet.util import pmonitor

import json


def generate_topo(n_hosts, host_band, host_queue, host_delay, host_loss):
    networks = {
        "s0": None
    }

    peers = {}

    for j in range(n_hosts):
        host = "h" + str(j)
        link = {
            "bw": host_band,
            "delay": host_delay,
            "loss": host_loss,
            "max_queue_size": host_queue
        }
        peers[host] = {"s0": link}

    return {
        "networks": networks,
        "peers": peers
    }


class CustomTopo(Topo):
    """Single switch connected to n hosts."""

    def build(self, net_topo):

        for net in net_topo['networks'].keys():
            self.addSwitch(net)
            if net_topo['networks'][net] is not None:
                for net_ in net_topo['networks'][net].keys():
                    args = net_topo['networks'][net][net_]
                    self.addLink(net, net_, **args)

        for peer in net_topo['peers'].keys():
            self.addHost(peer)
            for net_ in net_topo['peers'][peer].keys():
                args = net_topo['peers'][peer][net_]
                self.addLink(peer, net_, **args)


def run_test(input_path, transactions, transaction_delay, simulation_time):
    call(["mn", "-c"])

    with open(input_path, 'r') as input_file:
        input_data = input_file.read()

    input_obj = json.loads(input_data)

    number_of_nodes = input_obj['parameters']['n']

    full_topo = generate_topo(n_hosts=number_of_nodes+1, host_band=10,
                              host_queue=10000, host_delay="5ms", host_loss=1)

    topo = CustomTopo(full_topo)
    net = Mininet(topo, link=TCULink)

    try:
        net.start()

        print("Dumping switch connections")
        dumpNodeConnections(net.switches)

        print("Setting up main server")
        hosts = net.hosts

        popens = {}

        # server execution code
        cmd = "./mainserver --n " + str(number_of_nodes) + " --log_file outputs/mainOut.txt"
        popens[hosts[0]] = hosts[0].popen(cmd)

        sleep(.1)

        print("Setting up nodes")

        for i in range(number_of_nodes):
            cmd = f"./node --input_file {input_path} --log_file outputs/process{i}.txt --i {i} " + \
                  f"--transactions {transactions} --transaction_init_timeout_ns {transaction_delay} 2>&1"

            popens[hosts[i + 1]] = hosts[i + 1].popen(cmd, shell=True)

        print("Simulation start... ")
        end_time = time() + simulation_time

        for h, line in pmonitor(popens, timeoutms=100):
            if h:
                info('<%s>: %s' % (h.name, line))
            if time() >= end_time:
                print("Sending termination signal...")
                for p in popens.values():
                    p.send_signal(SIGINT)
                break
        print("Simulation end... ")
        net.stop()
    finally:
        call("sudo pkill -f node", shell=True)
        call("sudo pkill -f mainserver", shell=True)
        call(["mn", "-c"])

        print("The end")


if __name__ == '__main__':
    setLogLevel('info')
    input_dir = sys.argv[1]

    ns = [16, 32, 64, 80, 100]
    protocols = ["bracha", "witness", "scalable"]
    target_rate = [2 * (i + 1) for i in range(64)]

    total_time = 180000000000
    base_delay = 1000000000
    for protocol in protocols:
        for n in ns:
            for rate in target_rate:
                transaction_delay = (n / rate) * base_delay
                transactions = total_time / transaction_delay

                run_test(
                    input_path=f"{input_dir}/{protocol}_{n}.json",
                    transactions=int(transactions),
                    transaction_delay=int(transaction_delay),
                    simulation_time=int(total_time / base_delay) + 120
                )
