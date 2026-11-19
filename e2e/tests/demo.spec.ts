import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

async function ready(page: Page) {
  await page.goto('./');
  await expect(page.locator('#sample')).toBeEnabled({ timeout: 20_000 }); // WebAssembly загрузился
}

test('демо-лог: 1613 запросов, всплеск и пачка ошибок найдены', async ({ page }) => {
  await ready(page);
  await page.locator('#sample').click();
  await expect(page.locator('#stats')).toContainText('1613');
  await expect(page.locator('#an')).toContainText('traffic spike');
  await expect(page.locator('#an')).toContainText('5xx');
  await expect(page.locator('#top')).toContainText('GET /api/products/:id'); // пути с id объединены
  await expect(page.locator('#codes')).toContainText('5xx');
  await expect(page.locator('#lat')).toContainText('p99');
});

test('свой лог nginx: считает запросы и пропущенные строки', async ({ page }) => {
  await ready(page);
  await page.locator('#log').fill(
    [
      '203.0.113.5 - - [10/Oct/2026:13:55:36 +0000] "GET /a HTTP/1.1" 200 100 "-" "ua" 0.050',
      '203.0.113.5 - - [10/Oct/2026:13:55:37 +0000] "POST /b HTTP/1.1" 500 10 "-" "ua" 0.250',
      'это не лог',
    ].join('\n'),
  );
  await page.locator('#run').click();
  await expect(page.locator('#stats')).toContainText('Requests');
  await expect(page.locator('#stats')).toContainText('2');
  await expect(page.locator('#stats')).toContainText('Skipped lines');
  await expect(page.locator('#top')).toContainText('GET /a');
  await expect(page.locator('#top')).toContainText('POST /b');
});

test('JSON-строки тоже понимает', async ({ page }) => {
  await ready(page);
  await page.locator('#log').fill('{"time":"2026-10-10T13:55:36Z","method":"GET","path":"/x","status":200,"duration_ms":12}');
  await page.locator('#run').click();
  await expect(page.locator('#top')).toContainText('GET /x');
});

test('пустой ввод и мусор не ломают страницу', async ({ page }) => {
  await ready(page);
  await page.locator('#run').click();
  await expect(page.locator('#msg')).toContainText('Paste some log lines');
  await page.locator('#log').fill('полный мусор\nещё строка');
  await page.locator('#run').click();
  await expect(page.locator('#stats')).toContainText('Skipped lines');
});

test('нет горизонтальной прокрутки на узком экране', async ({ page }) => {
  await ready(page);
  await page.setViewportSize({ width: 360, height: 740 });
  await page.locator('#sample').click();
  await expect(page.locator('#stats')).toContainText('1613');
  expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(0);
});

for (const scheme of ['light', 'dark'] as const) {
  test(`доступность (axe), ${scheme === 'light' ? 'светлая' : 'тёмная'} тема`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme });
    await ready(page);
    const check = async (label: string) => {
      const res = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
      const bad = res.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
      expect(bad.map((v) => `${v.id}: ${v.nodes[0]?.html}`), label).toEqual([]);
    };
    await check('начальный экран');
    await page.locator('#sample').click();
    await expect(page.locator('#stats')).toContainText('1613');
    await check('результаты анализа');
  });
}
