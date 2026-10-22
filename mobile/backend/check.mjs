import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { connectionSchema, listSchema, quoteSchema, accountSchema, portfolioSchema } from '../src/api/schemas.ts';
const config = Object.fromEntries(readFileSync(new URL('../../.env', import.meta.url), 'utf8').split(/\r?\n/).flatMap(line => { const m=line.match(/^(SEED_ADMIN_LOGIN|SEED_ADMIN_PASSWORD)=(.*)$/); return m ? [[m[1],m[2].replace(/^['"]|['"]$/g,'')]] : []; }));
const origin=process.env.SBIS_CHECK_ORIGIN || 'http://127.0.0.1:18080';
assert.ok(['http://127.0.0.1:18080','http://192.168.20.96:18080'].includes(origin),'Only the configured local broker API can be probed');
const login=await fetch(`${origin}/v1/auth/login`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({login:config.SEED_ADMIN_LOGIN,password:config.SEED_ADMIN_PASSWORD}),signal:AbortSignal.timeout(15000)});
assert.equal(login.status,200,'Existing broker login');
const token=await login.json(); const headers={Authorization:`Bearer ${token.access_token}`};
async function get(path,schema){const r=await fetch(origin+path,{headers,signal:AbortSignal.timeout(20000)});assert.equal(r.status,200,path);return schema.parse(await r.json());}
try {
 const settingsResponse=await fetch(origin+'/v1/integrations/kfb/settings',{headers,signal:AbortSignal.timeout(15000)});assert.equal(settingsResponse.status,200);
 const settings=await settingsResponse.json();assert.equal(settings.environment,'test');assert.equal(settings.rest_url,'http://host.docker.internal:8080');assert.equal(settings.grpc_target,'host.docker.internal:8443');
 const health=await get('/v1/integrations/kfb/health',connectionSchema);assert.equal(health.environment,'test');assert.equal(health.connected,true);assert.equal(health.read_only,false);
 const adaptersResponse=await fetch(origin+'/v1/adapters',{headers,signal:AbortSignal.timeout(15000)});assert.equal(adaptersResponse.status,200);
 const adapters=await adaptersResponse.json();assert.ok(adapters.items.some(row=>row.code==='KFB'&&row.status==='ONLINE'));
 assert.ok(adapters.online_runtime>=3,'KFB must join the two built-in local adapters after loading its saved profile');
 const market=await get('/v1/market',listSchema(quoteSchema));assert.ok(market.items.length>0);assert.ok(market.items.every(row=>row.source==='KFB'));
 const accounts=await get('/v1/accounts',listSchema(accountSchema));
 for(const account of accounts.items){const path=`/v1/accounts/${account.id}/exchange-portfolio`;const portfolio=await get(path,portfolioSchema);assert.equal(portfolio.source,'KFB');assert.equal((await fetch(origin+path)).status,401);}
 // No identifiers or parameters: submitOrder rejects this before touching the DB or exchange.
 const invalid=await fetch(origin+'/v1/orders',{method:'POST',headers:{...headers,'Content-Type':'application/json'},body:'{}',signal:AbortSignal.timeout(15000)});
 assert.equal(invalid.status,422,'Order endpoint accepts requests and validates the required fields');
 console.log(`PASS: local test KFB online; ${market.items.length} exchange instruments; ${accounts.items.length} scoped portfolios; anonymous access denied; order validation enabled. No orders submitted.`);
} finally {await fetch(origin+'/v1/auth/logout',{method:'POST',headers}).catch(()=>{});}
