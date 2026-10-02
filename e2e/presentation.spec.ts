import {test as base,expect} from './fixtures/bean'

const test=base.extend<{}, {appName:string}>({appName:['presentation',{scope:'worker'}]})

test('Bean introduction is a navigable data-backed eight-chapter presentation',async({page,bean})=>{
  await page.goto(bean.baseURL+'/presentations/bean')
  await expect(page.getByRole('main')).toHaveAttribute('data-profile','presentation')
  await expect(page.getByTestId('application-shell')).toHaveAttribute('data-accent','indigo')
  await expect(page.getByRole('navigation',{name:'Primary navigation'})).toHaveCount(0)
  await expect(page.getByLabel('1 of 19: Bean')).toBeVisible()
  await expect(page.getByText('1.1 / 8')).toBeVisible()

  await page.getByRole('button',{name:'Speaker notes'}).click()
  await expect(page.getByRole('heading',{name:'Speaker notes'})).toBeVisible()
  await page.getByRole('button',{name:'Down'}).click()
  await expect(page).toHaveURL(/frame=thesis/)
  await expect(page.getByLabel('2 of 19: Why deterministic semantics')).toBeVisible()
  await expect(page.getByText('1.2 / 8')).toBeVisible()

  await page.getByRole('button',{name:'Next'}).click()
  await expect(page.getByLabel('3 of 19: One path to production')).toBeVisible()
  await page.getByRole('button',{name:'Down'}).click()
  await expect(page.getByLabel('4 of 19: A small, complete vocabulary')).toBeVisible()
  await page.getByLabel('4 of 19: A small, complete vocabulary').click()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByLabel('6 of 19: From live data to authorized action')).toBeVisible()

  await page.keyboard.press('End')
  await expect(page).toHaveURL(/frame=start/)
  await expect(page.getByLabel('19 of 19: Start with one useful workflow')).toBeVisible()

  await page.getByLabel('Choose frame').selectOption('capabilities')
  await expect(page).toHaveURL(/frame=capabilities/)
  await expect(page.getByLabel('7 of 19: Live data, same runtime')).toBeVisible()
  await expect(page.getByTestId('bar-chart')).toBeVisible()
  for(const area of ['application','data','operations','safety'])await expect(page.getByLabel(`${area}: 3`)).toBeVisible()

  await page.reload()
  await expect(page.getByLabel('7 of 19: Live data, same runtime')).toBeVisible()
})

test('semantic content, tabs, choices, and media keep their browser contracts',async({page,bean})=>{
  const providerRequests:string[]=[]
  const browserRequests:string[]=[]
  page.on('request',request=>browserRequests.push(request.url()))
  page.on('request',request=>{if(request.url().startsWith('https://www.youtube-nocookie.com/'))providerRequests.push(request.url())})
  await page.route('https://www.youtube-nocookie.com/**',route=>route.fulfill({contentType:'text/html',body:'<!doctype html><title>fixture player</title>'}))
  await page.goto(bean.baseURL+'/presentations/bean?frame=static_table')
  await page.waitForLoadState('networkidle')

  const table=page.getByRole('region',{name:'Read and write boundaries'})
  await expect(table).toBeVisible()
  await expect(page.getByRole('columnheader',{name:'Primitive'})).toBeVisible()
  await expect(page.getByRole('rowheader',{name:'View'})).toBeVisible()
  await table.focus()
  await page.keyboard.press('ArrowRight')
  await expect(page).toHaveURL(/frame=static_table/)
  await page.setViewportSize({width:390,height:844})
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth)).toBe(true)

  await Promise.all([
    page.waitForResponse(response=>response.url().includes('/api/system/page?frame=tabs_horizontal')),
    page.getByLabel('Choose frame').selectOption('tabs_horizontal'),
  ])
  const requestsBeforeHorizontalTabs=browserRequests.length
  const horizontal=page.getByRole('tablist',{name:'Application boundaries'})
  const reads=horizontal.getByRole('tab',{name:'Reads'})
  const writes=horizontal.getByRole('tab',{name:'Writes'})
  await reads.focus()
  await page.keyboard.press('ArrowLeft')
  await expect(writes).toHaveAttribute('aria-selected','true')
  await expect(page).toHaveURL(/frame=tabs_horizontal/)
  await page.keyboard.press('Tab')
  await expect(page.getByRole('tabpanel')).toBeFocused()

  const tabQuiz=page.getByRole('group',{name:'Which primitive owns writes?'})
  const tabView=tabQuiz.getByRole('radio',{name:'View'})
  const tabAction=tabQuiz.getByRole('radio',{name:'Action'})
  await tabView.focus()
  await page.keyboard.press('ArrowDown')
  await expect(tabAction).toBeChecked()
  await tabQuiz.getByRole('button',{name:'Check answer'}).click()
  await expect(tabQuiz.getByText('Correct')).toBeVisible()
  await reads.click()
  await writes.click()
  await expect(tabAction).not.toBeChecked()
  await expect(tabQuiz.getByText('Correct')).toHaveCount(0)
  expect(browserRequests).toHaveLength(requestsBeforeHorizontalTabs)

  await Promise.all([
    page.waitForResponse(response=>response.url().includes('/api/system/page?frame=tabs_vertical')),
    page.getByLabel('Choose frame').selectOption('tabs_vertical'),
  ])
  const requestsBeforeVerticalTabs=browserRequests.length
  const vertical=page.getByRole('tablist',{name:'Release boundaries'})
  const definition=vertical.getByRole('tab',{name:'Definition'})
  const compiler=vertical.getByRole('tab',{name:'Compiler'})
  const activation=vertical.getByRole('tab',{name:'Activation'})
  await definition.focus()
  await page.keyboard.press('ArrowRight')
  await expect(page).toHaveURL(/frame=tabs_vertical/)
  await expect(definition).toHaveAttribute('aria-selected','true')
  await page.keyboard.press('ArrowDown')
  await expect(compiler).toHaveAttribute('aria-selected','true')
  await page.keyboard.press('End')
  await expect(activation).toHaveAttribute('aria-selected','true')
  await page.keyboard.press('Home')
  await expect(definition).toHaveAttribute('aria-selected','true')
  await page.keyboard.press('ArrowUp')
  await expect(activation).toHaveAttribute('aria-selected','true')
  expect(browserRequests).toHaveLength(requestsBeforeVerticalTabs)

  await Promise.all([
    page.waitForResponse(response=>response.url().includes('/api/system/page?frame=choices')),
    page.getByLabel('Choose frame').selectOption('choices'),
  ])
  const requestsBeforeQuiz=browserRequests.length
  const quiz=page.getByRole('group',{name:'Bean allows writes through which primitive?'})
  await quiz.getByRole('radio',{name:'View'}).focus()
  await page.keyboard.press('ArrowDown')
  await expect(quiz.getByRole('radio',{name:'Action'})).toBeChecked()
  await quiz.getByRole('button',{name:'Check answer'}).click()
  await expect(quiz.getByText('Correct')).toBeVisible()
  await page.getByRole('button',{name:'Speaker notes'}).click()
  await expect(quiz.getByText('Correct')).toBeVisible()
  expect(browserRequests).toHaveLength(requestsBeforeQuiz)
  await page.getByLabel('Choose frame').selectOption('semantic_content')
  await expect(page).toHaveURL(/frame=semantic_content/)
  await expect(page.getByLabel('10 of 19: Semantic content vocabulary')).toBeVisible()
  await page.getByLabel('Choose frame').selectOption('choices')
  await expect(quiz.getByRole('radio',{name:'Action'})).not.toBeChecked()
  await expect(quiz.getByText('Correct')).toHaveCount(0)

  await page.getByLabel('Choose frame').selectOption('external_media')
  expect(providerRequests).toEqual([])
  await page.getByRole('button',{name:'Load YouTube video'}).click()
  const frame=page.locator('iframe[title="YouTube embedded-player demonstration"]')
  await expect(frame).toBeVisible()
  await expect.poll(()=>providerRequests).toEqual(['https://www.youtube-nocookie.com/embed/M7lc1UVf-VE?autoplay=0&controls=1&playsinline=1&cc_load_policy=1'])
  await page.getByLabel('Choose frame').selectOption('semantic_content')
  await expect(frame).toHaveCount(0)
  await page.getByLabel('Choose frame').selectOption('external_media')
  await expect(page.getByRole('button',{name:'Load YouTube video'})).toBeVisible()

  await page.getByLabel('Choose frame').selectOption('local_media')
  await expect(page.getByRole('img',{name:/Definitions pass through validation/})).toBeVisible()
  await page.getByRole('button',{name:'Load audio'}).click()
  await expect(page.locator('audio[src="/assets/bean-intro.wav"]')).toBeVisible()
  await page.getByLabel('Choose frame').selectOption('semantic_content')
  await expect(page.locator('audio')).toHaveCount(0)

  await page.getByLabel('Choose frame').selectOption('blackboard')
  await expect(page.getByLabel('17 of 19: The blackboard lesson')).toBeVisible()
  const lesson=page.getByRole('article',{name:'Worked example: the quadratic formula'})
  await expect(lesson.getByRole('heading',{name:'The formula'})).toBeVisible()
  const math=lesson.locator('math')
  await expect(math).toHaveCount(1)
  expect((await math.textContent()).replace(/\s/g,'')).toMatch(/^x=.+2a$/)
  await expect(lesson.getByText('x equals (−b ± √(b² − 4ac)) / 2a')).toBeVisible()
  await expect(lesson.getByText('(−b±√Δ)/2a',{exact:true})).toBeVisible()
  const callout=lesson.locator('.bean-content-callout[data-tone="warning"]')
  expect(await callout.evaluate(element=>getComputedStyle(element).color)).toBe('rgb(240, 195, 127)')
  await page.setViewportSize({width:390,height:844})
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth)).toBe(true)
  const board=await lesson.boundingBox()
  for(const item of await lesson.locator('.bean-content-diagram li > span:first-child').all()){
    const box=await item.boundingBox()
    expect(box&&box.x>=board.x&&box.x+box.width<=board.x+board.width,'diagram item is clipped inside the lesson').toBeTruthy()
  }
  await page.setViewportSize({width:1280,height:800})

  await page.getByLabel('Choose frame').selectOption('timeline')
  await expect(page.getByLabel('18 of 19: A literal timeline')).toBeVisible()
  const timeline=page.getByRole('article',{name:'From writing to declarative apps'})
  const entries=timeline.getByRole('listitem')
  await expect(entries).toHaveCount(5)
  await expect(entries.first()).toContainText('~3400 BCE')
  await expect(entries.nth(2)).toContainText('Industrial Revolution')
  await expect(entries.last()).toContainText('Day 1')
  expect(await timeline.locator('time').count()).toBe(0)
  await page.setViewportSize({width:390,height:844})
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth)).toBe(true)

  // Valid long unbroken tokens must wrap inside the card instead of overflowing.
  await timeline.locator('li').first().evaluate(item=>{
    item.querySelector('.bean-timeline-label').textContent='~'+('x'.repeat(78))+'B'
    item.querySelector('.bean-timeline-entry-title').textContent='y'.repeat(120)
    const description=item.querySelector('.bean-timeline-description')
    if(description)description.textContent='z'.repeat(400)
  })
  const card=await timeline.boundingBox()
  for(const text of await timeline.locator('.bean-timeline-label,.bean-timeline-entry-title,.bean-timeline-description').all()){
    expect(await text.evaluate(element=>element.scrollWidth<=element.clientWidth,'element has internal horizontal overflow')).toBeTruthy()
    const box=await text.boundingBox()
    expect(box&&box.x>=card.x&&box.x+box.width<=card.x+card.width,'text is clipped outside the card').toBeTruthy()
  }
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth)).toBe(true)
  await page.setViewportSize({width:1280,height:800})

  await page.getByLabel('Choose frame').selectOption('tabs_horizontal')
  await expect(page.getByRole('tab',{name:'Reads'})).toHaveAttribute('aria-selected','true')
  await page.emulateMedia({media:'print'})
  const hiddenPanel=page.locator('.bean-tabs-panel[data-hidden="true"]').first()
  expect(await hiddenPanel.evaluate(element=>getComputedStyle(element).display)).not.toBe('none')
})
