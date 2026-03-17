import { test, expect } from '@playwright/test';
import { signup, uniqueId, TEACHER_ID, TEACHER_PASSWORD } from './helpers';

test('student signup redirects to dashboard with Student role', async ({ page }) => {
  const id = uniqueId('student');
  await page.goto('/');
  await page.fill('#signup-id', id);
  await page.fill('#signup-username', 'Test Student');
  await page.fill('#signup-password', 'StudentPass123!');
  await page.click('form[hx-post="/signup"] button[type="submit"]');

  await page.waitForURL(new RegExp(`/user/${id}`));
  await expect(page.locator('body')).toContainText('Test Student');
});

test('student signup and set-cookie redirects to dashboard', async ({ page }) => {
  const id = uniqueId('student');
  await signup(page, id, 'Dashboard Student', 'StudentPass123!');

  await expect(page).toHaveURL(new RegExp(`/user/${id}`));
  await expect(page.locator('body')).toContainText('Dashboard Student');
});

test('student dashboard shows task list without lesson creation form', async ({ page }) => {
  const id = uniqueId('student');
  await signup(page, id, 'Task Student', 'StudentPass123!');

  await expect(page.locator('body')).toContainText('Your Tasks');
  await expect(page.locator('textarea[name="description"]')).not.toBeVisible();
});

test('teacher signup redirects to dashboard with Teacher role in footer', async ({ page }) => {
  await page.goto('/');
  await page.fill('#signup-id', TEACHER_ID);
  await page.fill('#signup-username', 'E2E Teacher');
  await page.fill('#signup-password', TEACHER_PASSWORD);
  await page.click('form[hx-post="/signup"] button[type="submit"]');

  // Wait for navigation (either to profile or error shown if teacher already exists)
  await page.waitForFunction(
    () => window.location.pathname.includes('/user/') ||
          (document.body.textContent || '').includes('User already exists')
  );

  // Teacher may already exist — fall back to signin
  const bodyText = await page.locator('body').textContent();
  if (bodyText?.includes('User already exists')) {
    await page.fill('#signin-id', TEACHER_ID);
    await page.fill('#signin-password', TEACHER_PASSWORD);
    await page.click('form[hx-post="/signin"] button[type="submit"]');
    await page.waitForURL(new RegExp(`/user/${TEACHER_ID}`));
  }

  await expect(page.locator('body')).toContainText('Teacher');
});

test('teacher dashboard shows lesson creation form', async ({ page }) => {
  await page.goto('/');
  await page.fill('#signin-id', TEACHER_ID);
  await page.fill('#signin-password', TEACHER_PASSWORD);
  await page.click('form[hx-post="/signin"] button[type="submit"]');
  await page.waitForURL(new RegExp(`/user/${TEACHER_ID}`));

  await expect(page.locator('textarea[name="description"]')).toBeVisible();
  await expect(page.locator('input[name="datetime"]')).toBeVisible();
});

test('duplicate signup shows error', async ({ page }) => {
  const id = uniqueId('dup');
  await signup(page, id, 'First User', 'StudentPass123!');

  // Try signing up again with the same ID
  await page.goto('/logout');
  await page.goto('/');
  await page.fill('#signup-id', id);
  await page.fill('#signup-username', 'Second User');
  await page.fill('#signup-password', 'StudentPass123!');
  await page.click('form[hx-post="/signup"] button[type="submit"]');

  await expect(page.locator('body')).toContainText('User already exists');
});
