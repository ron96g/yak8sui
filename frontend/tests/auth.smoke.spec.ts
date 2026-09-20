import { expect, test } from '@playwright/test'

test('sign in, inspect resource YAML, switch tenants, and log out', async ({
  page,
}) => {
  test.setTimeout(60_000)
  const eventRequests: string[] = []
  const relationshipStreamRequests: string[] = []
  const resourceListRequests: string[] = []
  page.on('request', (request) => {
    if (request.url().includes('/events')) eventRequests.push(request.url())
    if (request.url().includes('/relationship-streams/'))
      relationshipStreamRequests.push(request.url())
    const url = new URL(request.url())
    if (
      request.method() === 'GET' &&
      url.pathname.includes('/api/v1/tenants/') &&
      url.pathname.includes('/resources/')
    )
      resourceListRequests.push(request.url())
  })
  await page.goto('/')
  await page.getByRole('link', { name: 'Sign in with OIDC' }).click()
  await expect(
    page.getByRole('heading', { name: 'Choose where to inspect' }),
  ).toBeVisible()
  await page.getByRole('combobox', { name: 'Tenant' }).click()
  await expect(page.getByRole('option', { name: 'Payments' })).toBeVisible()
  await page.getByRole('option', { name: 'Platform' }).click()
  const navigation = page.getByRole('navigation', { name: 'Resource types' })
  await expect(navigation.getByText('widgets.example.test/v1')).toBeVisible()
  await expect(
    navigation.getByText('policies.example.test/v1beta1'),
  ).toBeVisible()
  await navigation
    .getByRole('button', { name: 'widgets.example.test/v1' })
    .click()
  await expect(
    navigation.getByRole('button', { name: /Widget Namespaced/ }),
  ).toBeHidden()
  await navigation
    .getByRole('button', { name: 'widgets.example.test/v1' })
    .click()
  await expect(
    navigation.getByRole('button', { name: /Widget Namespaced/ }),
  ).toBeVisible()
  await navigation.getByRole('button', { name: /Widget Namespaced/ }).click()
  await expect(
    page.getByRole('table', { name: 'Widget resources' }),
  ).toBeVisible()
  await expect(
    page.getByRole('columnheader', { name: 'Created at' }),
  ).toBeVisible()
  await expect(
    page.getByRole('row', { name: /platform-api/ }).getByRole('time'),
  ).toBeVisible()
  await expect(
    page.getByRole('textbox', { name: 'Name', exact: true }),
  ).toBeEnabled()
  await expect(
    page.getByRole('textbox', { name: 'Namespace', exact: true }),
  ).toBeEnabled()
  const jsonPath = page.getByRole('textbox', { name: 'JSONPath' })
  await expect(jsonPath).toBeEnabled()
  await expect(page.getByText('platform-api', { exact: true })).toBeVisible()
  await expect(
    page.getByRole('row', { name: /platform-api/ }).getByText('Ready: Healthy'),
  ).toBeVisible()
  await expect(page.getByText('Stream: Live', { exact: true })).toBeVisible()
  const observationLoss = await page.request.post(
    '/demo/simulate/observation-loss',
  )
  expect(observationLoss.ok()).toBeTruthy()
  await expect(page.getByText(/Observation: unavailable/)).toBeVisible()
  await expect(page.getByText('platform-api', { exact: true })).toBeVisible()
  await expect(
    page.getByText('Stream: Disconnected — reconnecting…'),
  ).toBeVisible()
  await expect(page.getByText('Stream: Live', { exact: true })).toBeVisible()
  await expect(page.getByText('platform-api', { exact: true })).toHaveCount(0)
  await expect(page.getByText('105 resources · Page 1 of 2')).toBeVisible()
  const restoreObservation = await page.request.post(
    '/demo/simulate/observation-loss',
  )
  expect(restoreObservation.ok()).toBeTruthy()
  await expect(page.getByText(/Observation: unavailable/)).toBeVisible()
  await expect(page.getByText('Stream: Live', { exact: true })).toBeVisible()
  await expect(page.getByText('platform-api', { exact: true })).toBeVisible()
  await expect(page.getByText('payments-worker', { exact: true })).toHaveCount(
    0,
  )
  await expect(page.getByText('internal-only', { exact: true })).toHaveCount(0)

  await jsonPath.fill('$.spec.replicas')
  await expect(page.getByText('platform-api', { exact: true })).toBeVisible()
  await page.getByLabel('JSON comparison').click()
  await page.getByRole('option', { name: 'Equals' }).click()
  const jsonValue = page.getByRole('textbox', { name: 'JSON value' })
  await jsonValue.fill('null')
  await expect(
    page.getByText('No resources match these filters.'),
  ).toBeVisible()
  await jsonValue.fill('3')
  await expect(page.getByText('platform-api', { exact: true })).toBeVisible()
  await jsonValue.fill('not-json')
  await expect(
    page.getByText(/value must be one JSON string, number, boolean, or null/),
  ).toBeVisible()
  await jsonValue.fill('3')
  await expect(page.getByText('platform-api', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Clear JSON filter' }).click()
  await expect(jsonPath).toHaveValue('')

  const nameInput = page.getByRole('textbox', { name: 'Name', exact: true })
  await nameInput.pressSequentially('ps104', { delay: 25 })
  await page.getByRole('textbox', { name: 'Namespace', exact: true }).fill('sl')
  await expect(page.getByRole('button', { name: 'Apply' })).toHaveCount(0)
  await expect(
    page.getByText('platform-search-104', { exact: true }),
  ).toBeVisible()
  await expect(page.getByText('1 resource · Page 1 of 1')).toBeVisible()
  const combinedSearchRequests = resourceListRequests.filter((requestURL) => {
    const url = new URL(requestURL)
    return (
      url.searchParams.get('name') === 'ps104' &&
      url.searchParams.get('namespace') === 'sl'
    )
  })
  expect(combinedSearchRequests).toHaveLength(1)
  expect(
    resourceListRequests.filter((requestURL) => {
      const url = new URL(requestURL)
      return (
        url.searchParams.get('name')?.startsWith('ps') &&
        url.searchParams.get('name') !== 'ps104'
      )
    }),
  ).toHaveLength(0)
  const liveExit = await page.request.post('/demo/simulate/search-update')
  expect(liveExit.ok()).toBeTruthy()
  await expect(
    page.getByText('No resources match these filters.'),
  ).toBeVisible()
  await expect(page.getByText('0 resources · Page 1 of 1')).toBeVisible()
  const liveEntry = await page.request.post('/demo/simulate/search-update')
  expect(liveEntry.ok()).toBeTruthy()
  await expect(
    page.getByText('platform-search-104', { exact: true }),
  ).toBeVisible()
  await nameInput.fill('ps104-pending')
  const pendingLiveExit = await page.request.post(
    '/demo/simulate/search-update',
  )
  expect(pendingLiveExit.ok()).toBeTruthy()
  await expect(
    page.getByText('No resources match these filters.'),
  ).toBeVisible()
  await expect(nameInput).toHaveValue('ps104-pending')
  const pendingLiveEntry = await page.request.post(
    '/demo/simulate/search-update',
  )
  expect(pendingLiveEntry.ok()).toBeTruthy()
  await page.getByLabel('Name mode').click()
  await page.getByRole('option', { name: 'Regex' }).click()
  await page.getByRole('textbox', { name: 'Name', exact: true }).fill('(')
  await expect(page.getByText(/invalid Go regular expression/)).toBeVisible()
  await page
    .getByRole('textbox', { name: 'Name', exact: true })
    .fill('(?i)^platform-search')
  await page.getByRole('textbox', { name: 'Namespace', exact: true }).fill('')
  await expect(page.getByText('105 resources · Page 1 of 2')).toBeVisible()
  await page.getByRole('button', { name: 'Next' }).click()
  await expect(
    page.getByText('platform-search-100', { exact: true }),
  ).toBeVisible()
  await expect(page.getByText('105 resources · Page 2 of 2')).toBeVisible()

  await jsonPath.fill('$.spec.replicas')
  await page.getByLabel('JSON comparison').click()
  await page.getByRole('option', { name: 'Equals' }).click()
  await jsonValue.fill('1')
  await expect(page.getByText('105 resources · Page 1 of 2')).toBeVisible()
  await page.getByRole('button', { name: 'Clear JSON filter' }).click()

  await page
    .getByRole('textbox', { name: 'Name', exact: true })
    .fill('^platform-search-104$')
  await expect(page.getByText('1 resource · Page 1 of 1')).toBeVisible()
  await page.getByRole('textbox', { name: 'Name', exact: true }).fill('')
  await expect(page.getByText('106 resources · Page 1 of 2')).toBeVisible()

  await navigation.getByRole('button', { name: /Widget Namespaced/ }).click()
  await expect(page.getByLabel('Name mode')).toHaveText(/Regex/)
  await expect(page.getByText('106 resources · Page 1 of 2')).toBeVisible()

  await nameInput.fill('must-not-submit')

  await navigation
    .getByRole('button', { name: /ClusterPolicy Cluster/ })
    .click()
  await expect(
    page.getByRole('heading', { name: 'ClusterPolicy' }),
  ).toBeVisible()
  await expect(
    page.getByText('Cluster-scoped · Read-only inspection'),
  ).toBeVisible()
  await expect(
    page.getByRole('textbox', { name: 'Namespace', exact: true }),
  ).toBeDisabled()
  await expect(
    page.getByText('platform-guardrails', { exact: true }),
  ).toBeVisible()
  await page.waitForTimeout(350)
  expect(
    resourceListRequests.filter(
      (requestURL) =>
        new URL(requestURL).searchParams.get('name') === 'must-not-submit',
    ),
  ).toHaveLength(0)
  await expect(
    page.getByText('payments-guardrails', { exact: true }),
  ).toHaveCount(0)
  await page.getByText('platform-guardrails', { exact: true }).click()
  await expect(
    page.getByRole('dialog', {
      name: 'platform-guardrails — Resource details',
    }),
  ).toBeVisible()
  await expect(
    page.getByText('Unknown — determined only from the Ready condition.'),
  ).toBeVisible()
  await expect(
    page.getByText('No readable conditions were reported.'),
  ).toBeVisible()
  await expect(page.getByText(/Cluster scoped · platform-policy/)).toBeVisible()
  await page.getByRole('button', { name: 'Close', exact: true }).click()

  await navigation.getByRole('button', { name: /Widget Namespaced/ }).click()
  await page.getByRole('textbox', { name: 'JSONPath' }).fill('$.spec.replicas')
  await page.getByLabel('JSON comparison').click()
  await page.getByRole('option', { name: 'Equals' }).click()
  await page.getByRole('textbox', { name: 'JSON value' }).fill('3')
  await page.getByText('platform-api', { exact: true }).click()
  await expect(
    page.getByRole('dialog', { name: 'platform-api — Resource details' }),
  ).toBeVisible()
  const conditions = page.getByRole('table', { name: 'Resource conditions' })
  await expect(conditions).toContainText('Available')
  await expect(conditions).toContainText('All replicas are available')
  await expect(conditions).toContainText('3')
  await page.getByRole('tab', { name: 'YAML' }).click()
  await expect(page.locator('.view-lines')).toContainText('replicas: 3')
  await page.getByRole('tab', { name: 'Ownership' }).click()
  const ownershipDialog = page.getByRole('dialog')
  await expect(
    ownershipDialog.getByRole('button', { name: 'Table', exact: true }),
  ).toHaveAttribute('aria-pressed', 'true')
  await expect
    .poll(() => relationshipStreamRequests.length)
    .toBeGreaterThanOrEqual(1)
  const streamsBeforePresentationSwitch = relationshipStreamRequests.length
  const hierarchyTable = ownershipDialog.getByRole('table', {
    name: 'Loaded ownership hierarchy',
  })
  await expect(hierarchyTable).toBeVisible()
  await expect(
    hierarchyTable.getByRole('columnheader', {
      name: 'Predecessor / relationship',
    }),
  ).toHaveCount(0)
  await expect(
    hierarchyTable.getByRole('columnheader', { name: 'Branch state' }),
  ).toHaveCount(0)
  await expect(
    hierarchyTable.getByRole('columnheader', { name: 'Depth', exact: true }),
  ).toHaveCount(0)
  const settingsRow = hierarchyTable.getByRole('row', {
    name: /platform-settings.*Owner/,
  })
  await expect(settingsRow).toBeVisible()
  await expect(settingsRow).toHaveAttribute('data-depth', '1')
  const runtimeRow = hierarchyTable.getByRole('row', {
    name: /platform-runtime.*Owned resource/,
  })
  await expect(runtimeRow).toContainText('Unknown')
  await expect(runtimeRow).toHaveAttribute('data-depth', '1')
  await hierarchyTable
    .getByRole('button', {
      name: 'Show owners of platform-settings',
      exact: true,
    })
    .click()
  await expect(
    hierarchyTable.getByRole('row', { name: /platform-guardrails.*Owner/ }),
  ).toBeVisible()
  await expect(
    hierarchyTable.getByRole('row', { name: /platform-guardrails.*Owner/ }),
  ).toHaveAttribute('data-depth', '2')
  await page.waitForTimeout(1_000)
  expect(relationshipStreamRequests).toHaveLength(
    streamsBeforePresentationSwitch + 1,
  )
  await hierarchyTable
    .getByRole('button', {
      name: 'Hide owners of platform-settings',
      exact: true,
    })
    .click()
  await expect(
    hierarchyTable.getByRole('row', { name: /platform-guardrails/ }),
  ).toHaveCount(0)
  await hierarchyTable
    .getByRole('button', {
      name: 'Show owners of platform-settings',
      exact: true,
    })
    .click()
  await expect(
    hierarchyTable.getByRole('row', { name: /platform-guardrails.*Owner/ }),
  ).toBeVisible()
  const streamsAfterExpansion = relationshipStreamRequests.length
  await ownershipDialog
    .getByRole('button', { name: 'Graph', exact: true })
    .click()
  await expect(
    ownershipDialog.getByRole('group', {
      name: 'platform-settings owns platform-api',
      exact: true,
    }),
  ).toBeVisible()
  await expect(
    ownershipDialog.getByRole('group', {
      name: 'platform-api owns platform-runtime',
      exact: true,
    }),
  ).toBeVisible()
  await expect(
    ownershipDialog.getByRole('group', {
      name: 'platform-api owns platform-routes',
      exact: true,
    }),
  ).toBeVisible()
  await expect(
    ownershipDialog.getByRole('group', {
      name: 'Selected resource platform-api',
      exact: true,
    }),
  ).toBeVisible()
  await expect(
    ownershipDialog.getByRole('group', {
      name: 'platform-guardrails owns platform-settings',
      exact: true,
    }),
  ).toBeVisible()
  await ownershipDialog
    .getByRole('button', { name: 'Table', exact: true })
    .click()
  await expect(
    hierarchyTable.getByRole('row', { name: /platform-guardrails.*Owner/ }),
  ).toBeVisible()
  await page.waitForTimeout(250)
  expect(relationshipStreamRequests).toHaveLength(streamsAfterExpansion)
  await ownershipDialog
    .getByRole('button', { name: 'Graph', exact: true })
    .click()
  await expect(
    ownershipDialog.getByRole('group', {
      name: 'Resource platform-guardrails',
      exact: true,
    }),
  ).toBeVisible()
  await ownershipDialog
    .getByRole('button', { name: 'Inspect platform-settings', exact: true })
    .click()
  await expect(
    page.getByRole('dialog', { name: 'platform-settings — Resource details' }),
  ).toBeVisible()
  await page.getByRole('tab', { name: 'YAML' }).click()
  await expect(page.locator('.view-lines')).toContainText('mode: safe')
  await page.getByRole('tab', { name: 'Ownership' }).click()
  await page.getByRole('button', { name: 'Graph', exact: true }).click()
  const relatedPlatformAPI = page
    .getByRole('dialog')
    .getByRole('group', { name: 'Resource platform-api', exact: true })
  await expect(relatedPlatformAPI).toBeVisible({ timeout: 10_000 })
  await page
    .getByRole('button', {
      name: 'Show resources owned by platform-api',
      exact: true,
    })
    .click()
  await expect(
    page.getByRole('dialog').getByRole('group', {
      name: 'platform-api owns platform-runtime',
      exact: true,
    }),
  ).toBeVisible()
  // Directional expansion does not repeat the selected parent as a false cycle.
  await expect(
    page.getByRole('dialog').getByText('Cycle — already in this chain'),
  ).toHaveCount(0)
  const ownershipUpdate = await page.request.post(
    '/demo/simulate/ownership-update',
  )
  expect(ownershipUpdate.ok()).toBeTruthy()
  await expect(relatedPlatformAPI).toHaveCount(0)
  await page.waitForTimeout(1_000)
  const ownershipRestore = await page.request.post(
    '/demo/simulate/ownership-update',
  )
  expect(ownershipRestore.ok()).toBeTruthy()
  await expect(relatedPlatformAPI).toBeVisible({ timeout: 10_000 })
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Inspect platform-api', exact: true })
    .click()
  await page.getByRole('tab', { name: 'YAML' }).click()
  const simulatedUpdate = await page.request.post(
    '/demo/simulate/widget-update',
  )
  expect(simulatedUpdate.ok()).toBeTruthy()
  await expect(page.locator('.view-lines')).toContainText('generation: 4')
  await expect(
    page.getByRole('button', { name: 'Inspect platform-api' }),
  ).toHaveCount(0)
  await expect(
    page.getByRole('dialog', { name: 'platform-api — Resource details' }),
  ).toBeVisible()
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: 'Clear JSON filter' }).click()
  await page.getByText('platform-api', { exact: true }).click()
  await page.getByRole('tab', { name: 'Conditions' }).click()
  await expect(
    page.getByText(/Stale — Ready last reported Healthy/),
  ).toBeVisible()
  await expect(
    page.getByText('Ready: Stale (reported Healthy)', { exact: true }),
  ).toBeVisible()
  await expect(conditions).toContainText(
    'The previous generation was available',
  )
  await expect(conditions).toContainText('Stale')
  const unhealthyUpdate = await page.request.post(
    '/demo/simulate/widget-update',
  )
  expect(unhealthyUpdate.ok()).toBeTruthy()
  await expect(
    page.getByText('Ready: Unhealthy', { exact: true }),
  ).toBeVisible()
  await expect(conditions).toContainText('ProgressDeadlineExceeded')
  await expect(conditions).toContainText(
    'Deployment is below its requested capacity',
  )
  await page.getByRole('tab', { name: 'YAML' }).click()
  await expect(page.locator('.view-lines')).toContainText('generation: 5')
  expect(eventRequests).toHaveLength(0)
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.getByRole('combobox', { name: 'Tenant' }).click()
  await page.getByRole('option', { name: 'Payments' }).click()
  await navigation.getByRole('button', { name: /Widget Namespaced/ }).click()
  await expect(page.getByText('payments-worker', { exact: true })).toBeVisible()
  await expect(
    page.getByText('Ready: Unhealthy', { exact: true }),
  ).toBeVisible()
  await expect(page.getByText('platform-api', { exact: true })).toHaveCount(0)
  await expect(page.getByText('internal-only', { exact: true })).toHaveCount(0)
  await page.getByText('payments-worker', { exact: true }).click()
  await expect(
    page.getByRole('table', { name: 'Resource conditions' }),
  ).toContainText('ProgressDeadlineExceeded')
  await page.getByRole('tab', { name: 'YAML' }).click()
  await expect(page.locator('.view-lines')).toContainText('status: "False"')
  await expect(page.locator('.view-lines')).toContainText(
    'ProgressDeadlineExceeded',
  )

  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await page.getByRole('button', { name: 'Log out' }).click()
  await expect(
    page.getByRole('link', { name: 'Sign in with OIDC' }),
  ).toBeVisible()
  await expect(page.getByText(/Active tenant:/)).toHaveCount(0)
  const response = await page.request.get('/api/session')
  expect(response.status()).toBe(401)
})
