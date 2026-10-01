import http from 'k6/http';
import {check} from 'k6';
export const options={
  scenarios:{spike:{executor:'ramping-arrival-rate',startRate:20,timeUnit:'1s',preAllocatedVUs:30,maxVUs:100,
    stages:[{target:20,duration:'30s'},{target:300,duration:'60s'},{target:500,duration:'90s'},{target:20,duration:'30s'}]}},
  thresholds:{http_req_failed:['rate<0.01'],http_req_duration:['p(95)<500'],checks:['rate>0.99']},
};
export default function(){const r=http.get(`${__ENV.BASE_URL}/api/events`);check(r,{'events available':r=>r.status===200});}
