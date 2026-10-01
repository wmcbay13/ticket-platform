import http from 'k6/http';
import {check} from 'k6';
export const options={scenarios:{booking:{executor:'constant-arrival-rate',rate:30,timeUnit:'1s',duration:__ENV.DURATION||'10m',preAllocatedVUs:10,maxVUs:30}}};
export function setup(){
  const events=http.get(`${__ENV.BASE_URL}/api/events`).json();
  const r=http.post(`${__ENV.BASE_URL}/api/orders`,JSON.stringify({event_id:events[0].id,quantity:1,payment_scenario:'success'}),{headers:{'Content-Type':'application/json','Idempotency-Key':`canary-${Date.now()}`}});
  if(r.status!==201)throw new Error(`Cannot create canary probe order: ${r.status}`);
  return {id:r.json().id};
}
export default function(data){const r=http.get(`${__ENV.BASE_URL}/api/orders/${data.id}`);check(r,{'booking available':r=>r.status===200});}
