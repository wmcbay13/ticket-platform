export interface Event {id:string; title:string; venue:string; starts_at:string; description:string; price_cents:number; capacity:number; available:number}
export interface Order {id:string; event_id:string; quantity:number; total_cents:number; status:'pending'|'confirmed'|'failed'; payment_scenario:string; created_at:string; updated_at:string}
export type Scenario = 'success'|'decline'|'retry'
export const money = (cents:number) => new Intl.NumberFormat('en-US',{style:'currency',currency:'USD',maximumFractionDigits:0}).format(cents/100)
export async function request<T>(path:string, init:RequestInit = {}):Promise<T> {
  const response = await fetch(path,init)
  const body = await response.json()
  if (!response.ok) throw new Error(body.error ?? 'The service is temporarily unavailable. Please try again.')
  return body as T
}
export function reserve(event:Event,quantity:number,scenario:Scenario,key:string) {
  return request<Order>('/api/orders',{method:'POST',headers:{'Content-Type':'application/json','Idempotency-Key':key},body:JSON.stringify({event_id:event.id,quantity,payment_scenario:scenario})})
}
