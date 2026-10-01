#!/usr/bin/env python3
import ipaddress,json,subprocess
from pathlib import Path
import yaml
network=json.loads(subprocess.check_output(['docker','network','inspect','kind']))[0]
subnets=[ipaddress.ip_network(x['Subnet']) for x in network['IPAM']['Config'] if ipaddress.ip_network(x['Subnet']).version==4]
if len(subnets)!=1:raise SystemExit('Expected one IPv4 subnet in the kind Docker network')
subnet=subnets[0]
if subnet.num_addresses<64:raise SystemExit('kind subnet is too small for the demo address pool')
start,end=subnet.broadcast_address-20,subnet.broadcast_address-11
allocated=[ipaddress.ip_interface(c['IPv4Address']).ip for c in network['Containers'].values() if c.get('IPv4Address')]
if any(start<=ip<=end for ip in allocated):raise SystemExit('Proposed MetalLB range overlaps a Docker container; configure a free range manually')
docs=[{'apiVersion':'metallb.io/v1beta1','kind':'IPAddressPool','metadata':{'name':'ticket-pool','namespace':'metallb-system'},'spec':{'addresses':[f'{start}-{end}']}},
      {'apiVersion':'metallb.io/v1beta1','kind':'L2Advertisement','metadata':{'name':'ticket-l2','namespace':'metallb-system'},'spec':{'ipAddressPools':['ticket-pool']}}]
Path('.local/metallb-pool.yaml').write_text(yaml.safe_dump_all(docs,sort_keys=False))
print(f'MetalLB pool: {start}-{end} on {subnet}')
