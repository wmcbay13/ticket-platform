#!/usr/bin/env python3
"""Export a measured demo chart. Requires matplotlib in a local virtualenv."""
import argparse
import datetime as dt
import json
import os
from pathlib import Path
from urllib.parse import urlencode
from urllib.request import urlopen

os.environ.setdefault('MPLCONFIGDIR', str(Path('.local/matplotlib').resolve()))
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

parser = argparse.ArgumentParser()
parser.add_argument('summary', type=Path)
parser.add_argument('--start', required=True, help='UTC ISO timestamp at load start')
parser.add_argument('--prometheus', default='http://127.0.0.1:9090')
parser.add_argument('--output', type=Path, default=Path('docs/evidence/spike'))
args = parser.parse_args()
start = dt.datetime.fromisoformat(args.start.replace('Z', '+00:00')).timestamp()
queries = {
    'requests_per_second': 'sum(rate(ticket_http_requests_total{service="catalog"}[30s]))',
    'catalog_replicas': 'kube_horizontalpodautoscaler_status_current_replicas{namespace="ticket",horizontalpodautoscaler="catalog"}',
    'catalog_cpu_cores': 'sum(rate(container_cpu_usage_seconds_total{namespace="ticket",container="catalog"}[1m]))',
}
data = {}
for name, query in queries.items():
    url = args.prometheus + '/api/v1/query_range?' + urlencode({'query': query, 'start': start, 'end': start + 600, 'step': 15})
    with urlopen(url, timeout=20) as response:
        result = json.load(response)
    assert result['status'] == 'success', result
    series = result['data']['result']
    assert len(series) == 1, f'{name}: expected one measured series, got {len(series)}'
    data[name] = series[0]['values']
summary = json.loads(args.summary.read_text())['metrics']
evidence = {'start_utc': args.start, 'source': 'Local kind / Prometheus; 15-second query steps',
            'k6': {key: summary[key] for key in ['http_reqs', 'http_req_duration', 'http_req_failed', 'iterations']},
            'prometheus': data}
args.output.parent.mkdir(parents=True, exist_ok=True)
args.output.with_suffix('.json').write_text(json.dumps(evidence, indent=2) + '\n')
plt.rcParams.update({'font.family': 'DejaVu Sans', 'font.size': 11, 'axes.spines.top': False, 'axes.spines.right': False})
fig, axes = plt.subplots(3, 1, figsize=(10, 7), sharex=True, layout='constrained')
title = f"{summary['http_reqs']['count']:,} requests · {summary['http_req_failed']['value']:.0%} failures · {summary['http_req_duration']['p(95)']:.2f} ms p95"
fig.suptitle('Ticket platform: measured local traffic spike\n' + title, fontsize=16, fontweight='bold')
for ax, name, label, color in zip(axes, queries, ['Catalog requests / sec', 'Catalog replicas', 'Catalog CPU cores'], ['#6a54ce', '#12845b', '#d8752a']):
    xs = [(float(t) - start) / 60 for t, _ in data[name]]
    ys = [float(v) for _, v in data[name]]
    ax.plot(xs, ys, color=color, linewidth=2, drawstyle='steps-post' if name == 'catalog_replicas' else 'default')
    ax.set_ylabel(label)
    ax.set_ylim(bottom=0)
    ax.grid(axis='y', alpha=.2)
    ax.axvspan(0, 3.5, color='#6a54ce', alpha=.06)
axes[1].set_yticks([0, 1, 2, 3, 4])
axes[-1].set_xlabel('Minutes from load start (shaded region: 210-second test)')
axes[-1].set_xlim(0, 10)
fig.savefig(args.output.with_suffix('.png'), dpi=160)
print(f'Evidence saved: {args.output}.json and .png')
