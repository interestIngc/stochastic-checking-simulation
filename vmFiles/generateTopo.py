#!/usr/bin/python                                                                            
import json
from topoCreation import createTopoFile

if __name__ == '__main__':
    
	# Read input file
	with open('input.json', 'r') as myfile:
		data = myfile.read()
		
	obj = json.loads(data)

	numberNodes = obj['parameters']['n'] + 1 # Including main server    
	
	createTopoFile(nHosts = numberNodes, netBand = 1000, hostDelay = '10ms', outFileName = 'networkTopo.json')
