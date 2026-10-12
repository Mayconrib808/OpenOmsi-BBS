// Local setup UI checks. No BCS, real company, or production update is contacted.
const {chromium}=require('playwright');
const {spawn,execFileSync}=require('node:child_process');
const {mkdtempSync,mkdirSync,readFileSync}=require('node:fs');
const os=require('node:os');
const path=require('node:path');

(async()=>{
 const stage=mkdtempSync(path.join(os.tmpdir(),'bbs-v3-ui-'));
 const binary=path.join(stage,'Setup');
 execFileSync('go',['build','-buildvcs=false','-o',binary,'.'],{cwd:path.resolve(__dirname,'../source')});
 const app=spawn(binary,['--setup']);
 let browser;
 try{
  const address=await new Promise((resolve,reject)=>{let text='';const timer=setTimeout(()=>reject(Error('Setup did not open')),10000);app.stdout.on('data',b=>{text+=b;const match=text.match(/http:\/\/127\.0\.0\.1:\d+\/\?token=[a-f0-9]+/);if(match){clearTimeout(timer);resolve(match[0])}});app.on('error',reject)});
  browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_PATH||'/usr/bin/google-chrome'});
  const page=await browser.newPage({viewport:{width:1200,height:1000}});
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(address);
  await page.locator('[name=language][value="en"]').check();
  await page.getByRole('button',{name:'Continuar · Continue · Weiter'}).click();
  await page.getByRole('heading',{name:'Your company, your multiplayer'}).waitFor();
  if(!await page.locator('#bcsstatus').textContent().then(v=>v.includes('has not been checked')))throw Error('Initial status was not translated');
  await page.locator('[name=room_mode][value=create]').check();
  if(!await page.locator('#name').isVisible()||await page.locator('#invite').isVisible())throw Error('Company mode failed');
  await page.locator('[name=automatic_updates]').uncheck();
  await page.locator('button[value=preferences]').click();
  if(await page.locator('[name=automatic_updates]').isChecked())throw Error('Update preference reset');
  await page.locator('button[value=checkupdate]').click();
  await page.waitForFunction(()=>{const s=document.getElementById('updatestatus').textContent;return s.includes('No newer 3.x')||s.includes('could not')||s.includes('available')},null,{timeout:15000});
  await page.getByRole('button',{name:'Disable multiplayer',exact:true}).click();
  await page.getByRole('button',{name:'Enable multiplayer',exact:true}).waitFor();
  if(await page.locator('#invite').evaluate(e=>e.required))throw Error('Solo still requires invitation');
  await page.getByRole('button',{name:'Disable program',exact:true}).waitFor();
  await page.locator('#language').selectOption('pt-BR');
  await page.locator('button[value=preferences]').click();
  await page.getByRole('heading',{name:'Sua empresa, seu multiplayer'}).waitFor();
  const previews=path.resolve(process.env.UI_PREVIEW_DIR||path.join(stage,'previews'));mkdirSync(previews,{recursive:true});
  await page.screenshot({path:path.join(previews,'setup-pt.png'),fullPage:true});
  await page.locator('#language').selectOption('de');
  await page.locator('button[value=preferences]').click();
  await page.getByRole('heading',{name:'Deine Firma, dein Multiplayer'}).waitFor();
  if(!await page.locator('#bcsstatus').textContent().then(v=>v.includes('noch nicht geprüft')))throw Error('Status did not follow language choice');
  if(!await page.locator('#updatestatus').textContent())throw Error('Missing update status');
  await page.setViewportSize({width:390,height:844});
  await page.screenshot({path:path.join(previews,'setup-de-mobile.png'),fullPage:true});
  if(await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth))throw Error('Mobile layout overflow');
  if(errors.length)throw Error(errors.join('\n'));
  const settings=JSON.parse(readFileSync(path.join(stage,'OpenOMSI-BBS/config/settings.json'),'utf8'));
  if(settings.automatic_updates!==false||settings.language!=='de')throw Error('Preferences were not saved');
  console.log(JSON.stringify({languages:['en','pt-BR','de'],manual_updates:true,automatic_choice_preserved:true,status_localized:true,page_errors:errors}));
 }finally{if(browser)await browser.close();app.kill()}
})().catch(e=>{console.error(e);process.exitCode=1});
