#!/usr/bin/env python3
"""Verify that a demo PostgreSQL pod restart preserves an order and inventory."""
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid

base = sys.argv[1].rstrip('/')
def call(path, body=None):
    request = urllib.request.Request(base + path, data=None if body is None else json.dumps(body).encode(),
        headers={'Content-Type': 'application/json', 'Idempotency-Key': str(uuid.uuid4())})
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.load(response)

event = call('/api/events')[0]
order = call('/api/orders', {'event_id': event['id'], 'quantity': 1, 'payment_scenario': 'success'})
path = '/api/orders/' + order['id']
deadline = time.monotonic() + 90
while time.monotonic() < deadline:
    order = call(path)
    if order['status'] != 'pending':
        break
    time.sleep(.5)
assert order['status'] == 'confirmed', order
inventory = call('/api/events/' + event['id'])['available']
kubectl = ['kubectl', '--context', 'kind-ticket-platform', '-n', 'ticket-data']
subprocess.run(kubectl + ['delete', 'pod', 'postgres-0', '--timeout=60s'], check=True)
subprocess.run(kubectl + ['rollout', 'status', 'statefulset/postgres', '--timeout=180s'], check=True)
deadline = time.monotonic() + 90
while True:
    try:
        restored = call(path)
        available = call('/api/events/' + event['id'])['available']
        break
    except (urllib.error.URLError, TimeoutError):
        if time.monotonic() >= deadline:
            raise
        time.sleep(1)
assert restored == order, (order, restored)
assert available == inventory, (inventory, available)
print('PostgreSQL pod restart preserved the confirmed order and inventory.')
