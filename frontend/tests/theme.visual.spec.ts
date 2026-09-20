import { expect, test } from '@playwright/test'

test('deep-space theme remains legible and keyboard-visible across key views', async ({
  page,
}, testInfo) => {
  const thirdPartyRequests: string[] = []
  page.on('request', (request) => {
    const url = new URL(request.url())
    if (url.hostname !== '127.0.0.1') thirdPartyRequests.push(request.url())
  })
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'YAK8SUI' })).toHaveCSS(
    'font-family',
    /Space Grotesk/,
  )
  await expect(page.locator('body')).toHaveCSS(
    'background-color',
    'rgb(10, 14, 23)',
  )
  await page.screenshot({ path: testInfo.outputPath('login-deep-space.png') })

  await page.getByRole('link', { name: 'Sign in with OIDC' }).click()
  await page.getByRole('combobox', { name: 'Tenant' }).click()
  await page.getByRole('option', { name: 'Platform' }).click()
  await expect(page.getByText('Stream: Live', { exact: true })).toBeVisible()
  await page
    .getByRole('navigation', { name: 'Resource types' })
    .getByRole('button', { name: /Widget Namespaced/ })
    .click()

  await page.getByRole('textbox', { name: 'Name', exact: true }).fill('pa')
  const inspect = page.getByRole('button', { name: 'Inspect platform-api' })
  await expect(inspect).toBeVisible()
  await inspect.focus()
  await expect(inspect).toBeFocused()
  await expect(inspect).toHaveCSS('outline-color', 'rgb(56, 189, 248)')
  await page.screenshot({ path: testInfo.outputPath('workspace-focus.png') })

  await inspect.click()
  await expect(
    page.getByRole('table', { name: 'Resource conditions' }),
  ).toBeVisible()
  await page.getByRole('dialog').screenshot({
    path: testInfo.outputPath('conditions-modal.png'),
  })
  await page.getByRole('tab', { name: 'Ownership' }).click()
  await expect(
    page.getByRole('group', {
      name: 'Resource platform-settings',
      exact: true,
    }),
  ).toBeVisible()
  await page
    .getByRole('button', {
      name: 'Expand entire hierarchy',
      exact: true,
    })
    .click()
  await expect(
    page.getByRole('dialog').getByRole('group', {
      name: 'platform-guardrails owns platform-settings',
      exact: true,
    }),
  ).toBeVisible()
  await expect(
    page.getByRole('button', {
      name: 'Hide resources owned by platform-runtime',
      exact: true,
    }),
  ).toHaveAttribute('aria-expanded', 'true')
  await expect(
    page.getByRole('button', {
      name: 'Hide owners of platform-guardrails',
      exact: true,
    }),
  ).toHaveAttribute('aria-expanded', 'true')
  await page.getByRole('dialog').screenshot({
    path: testInfo.outputPath('ownership-modal.png'),
  })
  const routes = page.getByRole('group', {
    name: 'Resource platform-routes',
    exact: true,
  })
  const runtime = page.getByRole('group', {
    name: 'Resource platform-runtime',
    exact: true,
  })
  await routes.scrollIntoViewIfNeeded()
  const routesBox = await routes.boundingBox()
  const runtimeBox = await runtime.boundingBox()
  expect(routesBox).not.toBeNull()
  expect(runtimeBox).not.toBeNull()
  expect(routesBox!.width).toBeLessThanOrEqual(260)
  expect(routesBox!.height).toBeLessThanOrEqual(72)
  expect(Math.abs(routesBox!.y - runtimeBox!.y)).toBeLessThan(2)
  expect(Math.abs(routesBox!.x - runtimeBox!.x)).toBeGreaterThanOrEqual(
    Math.min(routesBox!.width, runtimeBox!.width),
  )
  await page
    .getByRole('dialog')
    .screenshot({ path: testInfo.outputPath('ownership-siblings.png') })
  await page
    .getByRole('button', { name: 'Collapse branches', exact: true })
    .click()
  await expect(
    page.getByRole('group', {
      name: 'Resource platform-guardrails',
      exact: true,
    }),
  ).toHaveCount(0)
  await expect(
    page.getByRole('button', {
      name: 'Show owners of platform-settings',
      exact: true,
    }),
  ).toHaveAttribute('aria-expanded', 'false')
  await expect(routes).toBeVisible()
  await page.getByRole('tab', { name: 'YAML' }).click()
  await expect(page.locator('.view-lines')).toContainText('platform-api')
  await expect(page.locator('.monaco-editor')).toHaveCSS(
    'background-color',
    'rgb(10, 14, 23)',
  )
  await page.screenshot({ path: testInfo.outputPath('yaml-modal.png') })

  await page.keyboard.press('Escape')
  await page.setViewportSize({ width: 320, height: 800 })
  await expect(
    page.getByRole('navigation', { name: 'Resource types' }),
  ).toBeVisible()
  await expect(page.getByRole('button', { name: 'Log out' })).toBeVisible()
  await page.getByText('platform-api', { exact: true }).click()
  await page.getByRole('tab', { name: 'Ownership' }).click()
  await expect(
    page.getByRole('group', {
      name: 'Resource platform-settings',
      exact: true,
    }),
  ).toBeVisible()
  await page.getByRole('button', { name: 'Table', exact: true }).click()
  const narrowTable = page.getByRole('table', {
    name: 'Loaded ownership hierarchy',
  })
  await expect(narrowTable).toBeVisible()
  const tableViewport = narrowTable.locator('..')
  expect(
    await tableViewport.evaluate(
      (element) => element.scrollWidth > element.clientWidth,
    ),
  ).toBeTruthy()
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(320)
  await page
    .getByRole('dialog')
    .screenshot({ path: testInfo.outputPath('ownership-narrow.png') })
  await page.keyboard.press('Escape')
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(320)
  await page.screenshot({
    path: testInfo.outputPath('workspace-narrow.png'),
    fullPage: true,
  })
  expect(thirdPartyRequests).toEqual([])
})
