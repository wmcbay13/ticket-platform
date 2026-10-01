#!/usr/bin/env python3
import json,sys,time,urllib.request,uuid
base=sys.argv[1].rstrip('/')
def call(path,body=None,key=None):
    headers={'Content-Type':'application/json'}
    if key:headers['Idempotency-Key']=key
    req=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers=headers)
    with urllib.request.urlopen(req,timeout=10) as r:return json.load(r)
events=call('/api/events');assert len(events)>=3
for scenario,outcome in [('success','confirmed'),('decline','failed'),('retry','confirmed')]:
    key=str(uuid.uuid4());body={'event_id':events[0]['id'],'quantity':1,'payment_scenario':scenario}
    order=call('/api/orders',body,key);assert order['status']=='pending'
    duplicate=call('/api/orders',body,key);assert duplicate['id']==order['id']
    deadline=time.monotonic()+90
    while time.monotonic()<deadline:
        result=call('/api/orders/'+order['id'])
        if result['status']!='pending':break
        time.sleep(.5)
    assert result['status']==outcome,result
    print(f'{scenario}: pending -> {outcome}; idempotent replay preserved order')
print('Smoke checks passed.')
