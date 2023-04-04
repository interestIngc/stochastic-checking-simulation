#!/usr/bin/env python
# coding: utf-8


def createTopoFile(netBand = 10000, netQueue = 100000, nHosts = 10, hostBand = 10,
                   hostQueue = 1000, hostDelay = '50ms', hostLoss = 1,
                   latFilePath = 'latencyData.csv', dropDataPath = 'dropData.csv',
                   outFileName = 'verizonTopo.json'):
    
    '''
    	Creates a json file with network topology details using
        Verizon's data about latency across countries
        
        Bandwidth in netBand and hostBand are in Mb.
        
        hostLoss is in percentage.
    '''
    
    import pandas as pd
    import numpy as np
    import json
    
    # Reads latency data
    latencyData = pd.read_csv(latFilePath)

    headers = latencyData.loc[1].to_list()
    headers[0] = 'Countries'

    latencyData.set_axis(headers, axis=1, inplace=True)
    latencyData.drop([0,1], axis=0, inplace=True)

    # Reads drop data
    dropData = pd.read_csv(dropDataPath)

    dropData.set_axis(headers, axis=1, inplace=True)
    dropData.drop([0,1], axis=0, inplace=True)

    # Maintains only elements with network data across countries
    latencyData = latencyData[["to" in x for x in latencyData['Countries']]]
    dropData = dropData[["to" in x for x in dropData['Countries']]]

    months = latencyData.columns.to_list()[1:]

    # filter rows with missing data
    filt = latencyData['Countries'] != 0 # returns everything true

    for month in months:
        filt = filt * (latencyData[month] != '-')
        filt = filt * (dropData[month] != '-')

    latencyData = latencyData[filt]
    dropData = dropData[filt]

    # Converting drops and delays to float value
    for month in months:
        latencyData[month] = latencyData[month].astype(float)
        dropData[month] = dropData[month].astype(float)

    # Averaging drops and delays per country pair
    lat_avrgs = []
    drop_avrgs = []
    for i in dropData.index:
        lat_avrgs.append(np.mean(latencyData.loc[i][months]))
        drop_avrgs.append(np.mean(dropData.loc[i][months]))

    latencyData['Avrg'] = lat_avrgs
    dropData['Avrg'] = drop_avrgs

    # Extracting all remaining countries with data available
    countries = set()

    for line in dropData['Countries']:
        first, second = extractCountries(line)

        countries.add(first)
        countries.add(second)


    # Converting country-pair link information to dictionary format
    edges = {}

    for line in dropData['Countries']:
        first, second = extractCountries(line)

        filt = dropData['Countries'] == line
        index = dropData[filt].index[0]
        lat = latencyData['Avrg'].loc[index]
        drop = dropData['Avrg'].loc[index]

        edges[(first, second)] = {"delay": lat, "loss": 100 - drop}
        
    # Gets the spanning tree over (countries,edges)
    treeEdges = getSpanningTree(countries,edges)

    # Creates a dictionary with full information about the topology
    topo = createOutput(countries, treeEdges, netBand, netQueue,
                        nHosts, hostBand, hostQueue, hostDelay, hostLoss)

    # Serializing json
    json_object = json.dumps(topo, indent=4)

    # Writing to sample.json
    with open(outFileName, "w") as outfile:
        outfile.write(json_object)
        
#----------------------------------------------------------------------
    
def extractCountries(element):
    
    ''' Extract country names from database first column '''
    
    parsed_element = element.split(" ")[:-1]
    
    if "to" in parsed_element:
        first_country = []
        second_country = []
        
        reachedTo = False
        for word in parsed_element:
            if word == "to":
                reachedTo = True
                continue
                
            if not reachedTo:
                first_country.append(word)
            else:
                second_country.append(word)
                
        return ' '.join(fix_kong(first_country)), ' '.join(fix_kong(second_country))
                
    else:
        return '',''
    
def fix_kong(word):
    
    ''' Hong Kong came in two different names (with and without space),
        this function is used to normalize its name '''
    
    if 'Hongkong' in word:
        return ['Hong', 'Kong']
    else:
        return word
    
def createOutput(countries, edges, netBand, netQueue, nHosts, hostBand, hostQueue, hostDelay, hostLoss):
    
    ''' Creates a random distribution of hosts over the network topo defined by edges'''
    
    import random
    
    # Renaming countries to match mininet's notation
    i = 0
    names = {}
    for c in countries:
        names[c] = "s" + str(i)
        i += 1
        
    netOutput = {}
    hostOutput = {}
    output = {}
    
    done = set()
    for c1 in countries:
        done.add(c1)
        
        links = {}
        for c2 in done:
            link = getParam(c1, c2, edges)
            if len(link) != 0:
                links[names[c2]] = {
                    "bw": netBand,
                    "delay": str(int(link['delay'])) + "ms",
                    "loss": round(link['loss'],4),
                    "max_queue_size": netQueue
                }
                
        netOutput[names[c1]] = None
                
        if len(links) != 0:
            netOutput[names[c1]] = links
            
    output["networks"] = netOutput
    
    for j in range(nHosts):
        server = "s" + str(random.randrange(len(countries)))
        host = "h" + str(j)
        link = {
            "bw": hostBand,
            "delay": hostDelay,
            "loss": hostLoss,
            "max_queue_size": hostQueue
        }
        hostOutput[host] = {server: link}
        
    output["peers"] = hostOutput
    
    return output

def getParam(c1, c2, edges):
    
    ''' Identify and retrieve edges with country names '''
    
    if (c1,c2) in edges.keys():
        return edges[(c1,c2)]
    if (c2,c1) in edges.keys():
        return edges[(c2,c1)]
    return {}

def getSpanningTree(countries, edges):
    
    treeEdges = {}
    
    forest = set([frozenset([country]) for country in countries])
    
    for (c1,c2) in sortEdges(edges):
        set1 = findSet(c1, forest)
        set2 = findSet(c2, forest)
        
        if set1 != set2:
            forest.remove(set1)
            forest.remove(set2)
            forest.add(set1.union(set2))
            
            treeEdges[(c1,c2)] = edges[(c1,c2)]
            
        if len(forest) == 1:
            return treeEdges
        
    return treeEdges
    
    
def sortEdges(edges):
    sortedEdges = {pair: edges[pair]['delay'] for pair in edges.keys()}
    sortedEdges = {k:v for k,v in sorted(sortedEdges.items(), key=lambda item: item[1])}
    return sortedEdges

def findSet(country, forest):
    for tree in forest:
        if country in tree:
            return tree
        
    return False