#!/usr/bin/python                                                                            
                                   
from time import sleep
from signal import SIGINT
import json
                                                                                             
from mininet.topo import Topo
from mininet.net import Mininet
from mininet.util import dumpNodeConnections
from mininet.log import setLogLevel
from mininet.node import CPULimitedHost
from mininet.link import TCLink

from topoCreation import createTopoFile

class CustomTopo(Topo):
	"Single switch connected to n hosts."
	def build(self, pathTopoFile):
		
		# read topo file
		with open(pathTopoFile, 'r') as topoFile:
			net_topo =  topoFile.read()
			
		# parse file
		net_topo = json.loads(net_topo)
		
		switches = {}
		for net in net_topo['networks'].keys():
			switches[net] = self.addSwitch(net)
			
			if net_topo['networks'][net] != None:
				for net_ in net_topo['networks'][net].keys():
					args = net_topo['networks'][net][net_]
					self.addLink(net, net_,**args)
				
		for peer in net_topo['peers'].keys():
			self.addHost(peer)
			for net_ in net_topo['peers'][peer].keys():
				args = net_topo['peers'][peer][net_]
				self.addLink(peer, net_,**args)

def simpleTest():

	# Read input file
	with open('input.json', 'r') as myfile:
		data = myfile.read()
		
	obj = json.loads(data)

	numberNodes = obj['parameters']['n'] + 1 # Including main server
	
	pathTopo = 'networkTopo.json'

	"Create and test a simple network"
	topo = CustomTopo(pathTopo)
	net = Mininet(topo, host=CPULimitedHost, link=TCLink)
	net.start()

	#print( "Dumping host connections" )
	#dumpNodeConnections(net.hosts)

	print( "Dumping switch connections" )
	dumpNodeConnections(net.switches)
	
	print( "Setting up main server" )
	hosts = net.hosts

	popens = {}

	# server execution code
	cmd = "./mainserver --n " + str(numberNodes-1) + " --log_file outputs/mainOut.txt"
	popens[hosts[0]] = hosts[0].popen(cmd)
	print(cmd)

	sleep(1) # time to setup main server

	print( "Setting up nodes" )

	for i in range(1,numberNodes):
		# node execution code
		nodeId = str(i-1)
		cmd = "./node --input_file input.json --log_file outputs/process" + nodeId + ".txt --i " + nodeId + " --transactions 5 --transaction_init_timeout_ns 1000000000"
		popens[hosts[i]] = hosts[i].popen(cmd)
		#print(cmd)
		
	print("Simulation start... ")

	sleep(60)
	print("Simulation end... ")

	for p in popens.values():
		p.send_signal(SIGINT)
	
	sleep(10)

	print("The end")

	net.stop()

if __name__ == '__main__':
    # Tell mininet to print useful information
    setLogLevel('info')
    simpleTest()
