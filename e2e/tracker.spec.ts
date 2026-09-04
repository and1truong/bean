import {test as base,expect} from './fixtures/bean'
import {action,login} from './helpers/api'

const test=base.extend<{}, {appName:string}>({appName:['tracker',{scope:'worker'}]})

test('Page section widths, Panel presets, and collapsed Regions preserve responsive order',async({page,bean})=>{
  await page.setViewportSize({width:500,height:800})
  await page.goto(bean.baseURL+'/')
  await expect(page.getByRole('heading',{name:'Operate the issue lifecycle'})).toBeVisible()
  await expect(page.locator('section[data-component="Page"] > section[data-component="Panel"]').first()).toHaveAttribute('data-layout','single-column')
  await expect(page.locator('section[data-component="Page"] > section[data-component="Panel"]').first()).toHaveAttribute('data-page-width','contained')
  await expect(page.locator('section[data-component="Page"] > section[data-component="Panel"]').nth(1)).toHaveAttribute('data-layout','two-column')
  await expect(page.locator('section[data-component="Page"] > section[data-component="Panel"]').nth(1)).toHaveAttribute('data-page-width','wide')
  await page.evaluate(()=>{
    const fixture=document.createElement('div')
    fixture.id='panel-responsive-fixture'
    fixture.style.cssText='position:absolute;left:-10000px;top:0;width:min(900px,calc(100vw - 40px))'
    fixture.innerHTML=`
      <section id="single" class="bean-panel" data-component="Panel" data-layout="single-column"><section class="bean-region" data-component="Region" data-region="main"><div>one</div></section></section>
      <section id="two" class="bean-panel" data-component="Panel" data-layout="two-column"><section class="bean-region" data-component="Region" data-region="left"><div>left</div></section><section class="bean-region" data-component="Region" data-region="right"><div>right</div></section></section>
      <section id="sidebar-main" class="bean-panel" data-component="Panel" data-layout="sidebar-main"><section class="bean-region" data-component="Region" data-region="sidebar"><div>sidebar</div></section><section class="bean-region" data-component="Region" data-region="main"><div>main</div></section></section>
      <section id="collapsed-sidebar-main" class="bean-panel" data-component="Panel" data-layout="sidebar-main"><section class="bean-region" data-component="Region" data-region="main" data-expanded="true"><div>expanded main</div></section></section>
      <section id="main-sidebar" class="bean-panel" data-component="Panel" data-layout="main-sidebar"><section class="bean-region" data-component="Region" data-region="main"><div>main</div></section><section class="bean-region" data-component="Region" data-region="sidebar"><div>sidebar</div></section></section>
      <section id="grid" class="bean-panel" data-component="Panel" data-layout="grid"><section class="bean-region" data-component="Region" data-region="main"><div style="width:2000px">wide</div><div>two</div><div>three</div></section></section>
      <div id="page-widths" class="bean-page" style="width:100vw"><section id="width-contained" class="bean-panel" data-layout="single-column" data-page-width="contained"></section><section id="width-wide" class="bean-panel" data-layout="single-column" data-page-width="wide"></section><section id="width-full" class="bean-panel" data-layout="single-column" data-page-width="full"></section></div>`
    document.body.append(fixture)
  })
  const snapshot=()=>page.evaluate(()=>{
    const element=(selector:string)=>document.querySelector<HTMLElement>(selector)!
    const tracks=(selector:string)=>getComputedStyle(element(selector)).gridTemplateColumns.split(' ').filter(Boolean).length
    const sidebarMain=element('#sidebar-main'),sidebar=element('#sidebar-main > [data-region="sidebar"]'),main=element('#sidebar-main > [data-region="main"]')
    const collapsedPanel=element('#collapsed-sidebar-main'),expandedMain=element('#collapsed-sidebar-main > [data-region="main"]')
    const reverseMain=element('#main-sidebar > [data-region="main"]'),reverseSidebar=element('#main-sidebar > [data-region="sidebar"]')
    const grid=element('#grid'),gridRegion=element('#grid > [data-region="main"]')
    return {
      single:tracks('#single'),two:tracks('#two'),sidebarMain:tracks('#sidebar-main'),mainSidebar:tracks('#main-sidebar'),grid:tracks('#grid > [data-region="main"]'),
      sidebarMainOrder:Array.from(sidebarMain.children).map(child=>child.getAttribute('data-region')),
      mainSidebarOrder:Array.from(element('#main-sidebar').children).map(child=>child.getAttribute('data-region')),
      sidebarRatio:main.getBoundingClientRect().width/sidebar.getBoundingClientRect().width,
      expandedMainWidth:expandedMain.getBoundingClientRect().width,
      collapsedPanelWidth:collapsedPanel.getBoundingClientRect().width,
      mainSidebarRatio:reverseMain.getBoundingClientRect().width/reverseSidebar.getBoundingClientRect().width,
      regionMinWidth:getComputedStyle(gridRegion).minWidth,
      gridRegionWidth:gridRegion.getBoundingClientRect().width,
      gridPanelWidth:grid.getBoundingClientRect().width,
      containedWidth:element('#width-contained').getBoundingClientRect().width,
      wideWidth:element('#width-wide').getBoundingClientRect().width,
      fullWidth:element('#width-full').getBoundingClientRect().width,
    }
  })

  expect(await snapshot()).toMatchObject({single:1,two:1,sidebarMain:1,mainSidebar:1,grid:1,sidebarMainOrder:['sidebar','main'],mainSidebarOrder:['main','sidebar'],regionMinWidth:'0px'})
  await page.setViewportSize({width:800,height:800})
  expect(await snapshot()).toMatchObject({single:1,two:2,sidebarMain:1,mainSidebar:1,grid:2})
  await page.setViewportSize({width:1400,height:800})
  const large=await snapshot()
  const authoredWidths=await page.evaluate(()=>{
    const panels=Array.from(document.querySelectorAll<HTMLElement>('section[data-component="Page"] > section[data-component="Panel"]'))
    const chrome=document.querySelector<HTMLElement>('.bean-page-chrome')!
    return {intro:panels[0].getBoundingClientRect().width,operations:panels[1].getBoundingClientRect().width,chrome:chrome.getBoundingClientRect().width}
  })
  expect(large).toMatchObject({single:1,two:2,sidebarMain:3,mainSidebar:3,grid:3,sidebarMainOrder:['sidebar','main'],mainSidebarOrder:['main','sidebar'],regionMinWidth:'0px'})
  expect(large.sidebarRatio).toBeGreaterThan(1.9)
  expect(large.expandedMainWidth).toBeGreaterThan(large.collapsedPanelWidth-1)
  expect(large.mainSidebarRatio).toBeGreaterThan(1.9)
  expect(large.gridRegionWidth).toBeLessThanOrEqual(large.gridPanelWidth)
  expect(large.containedWidth).toBeCloseTo(768,0)
  expect(large.wideWidth).toBeCloseTo(1152,0)
  expect(large.fullWidth).toBeCloseTo(1400,0)
  expect(authoredWidths.intro).toBeCloseTo(768,0)
  expect(authoredWidths.operations).toBeCloseTo(1152,0)
  expect(authoredWidths.chrome).toBeCloseTo(1152,0)
})

test('issue status chart drills into actionable issue records',async({page,request,bean})=>{
  const csrf=await login(request,bean.baseURL)
  await action(request,bean.baseURL,csrf,'issue_create',{title:'Todo chart issue',status:'todo'})
  const active=await action(request,bean.baseURL,csrf,'issue_create',{title:'Active chart issue',status:'todo'})
  const activeID=(await active.json()).data.id
  expect((await action(request,bean.baseURL,csrf,'move_issue',{id:activeID,status:'in_progress'})).ok()).toBeTruthy()
  const done=await action(request,bean.baseURL,csrf,'issue_create',{title:'Done chart issue',status:'todo'})
  const doneID=(await done.json()).data.id
  expect((await action(request,bean.baseURL,csrf,'move_issue',{id:doneID,status:'in_progress'})).ok()).toBeTruthy()
  expect((await action(request,bean.baseURL,csrf,'move_issue',{id:doneID,status:'done'})).ok()).toBeTruthy()
  expect(await (await request.get(bean.baseURL+'/api/issues/kanban')).text()).toContain('Done chart issue')

  await page.goto(bean.baseURL+'/')
  await expect(page.getByRole('heading',{name:'Issue operations'})).toBeVisible()
  await expect(page.getByTestId('bar-chart')).toBeVisible()
  await expect(page.getByLabel('todo: 1')).toBeVisible()
  await page.getByRole('link',{name:'Open done records'}).click()
  await expect(page).toHaveURL(/\/issues\?status=done/)
  await expect(page.getByRole('cell',{name:'Done chart issue',exact:true})).toBeVisible()
  await expect(page.getByRole('cell',{name:'Todo chart issue',exact:true})).not.toBeVisible()
})
