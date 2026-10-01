import { afterEach, describe, expect, it, vi } from 'vitest'
import { request, reserve, type Event } from './api'
afterEach(()=>vi.unstubAllGlobals())
describe('API boundary',()=>{
  it('preserves the key and payload when retrying an ambiguous network failure',async()=>{
    const fetchMock=vi.fn().mockRejectedValueOnce(new TypeError('network error')).mockResolvedValueOnce(new Response(JSON.stringify({id:'same-order',status:'pending'}),{status:200}))
    vi.stubGlobal('fetch',fetchMock)
    const event={id:'neon-nights'} as Event
    await expect(reserve(event,2,'success','stable-key')).rejects.toThrow('network error')
    await expect(reserve(event,2,'success','stable-key')).resolves.toMatchObject({id:'same-order'})
    expect(fetchMock.mock.calls[0]).toEqual(fetchMock.mock.calls[1])
  })
  it('shows a sold-out error from the booking API',async()=>{
    vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response(JSON.stringify({error:'not enough tickets available'}),{status:409})))
    await expect(request('/api/orders')).rejects.toThrow('not enough tickets available')
  })
})
