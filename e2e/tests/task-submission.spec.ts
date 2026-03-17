import { test, expect } from '@playwright/test';
import { signup, uniqueId, TEACHER_ID, TEACHER_PASSWORD } from './helpers';

test('student submits task and sees it in history', async ({ page }) => {
  const id = uniqueId('student');
  await signup(page, id, 'Submitting Student', 'StudentPass123!');

  await page.goto(`/user/${id}/task/task1`);
  await expect(page.locator('body')).toContainText('No records for this task yet');

  await page.fill('textarea[name="content"]', 'My **markdown** submission');
  await page.click('button[type="submit"]');

  await page.waitForURL(`/user/${id}/task/task1`);
  await expect(page.locator('body')).toContainText('My markdown submission');
});

test('task record content is rendered as markdown', async ({ page }) => {
  const id = uniqueId('student');
  await signup(page, id, 'Markdown Student', 'StudentPass123!');

  await page.goto(`/user/${id}/task/task1`);
  await page.fill('textarea[name="content"]', '**bold text** and _italic_');
  await page.click('button[type="submit"]');

  await page.waitForURL(`/user/${id}/task/task1`);
  // Rendered HTML should contain <strong> not raw **
  await expect(page.locator('.markdown').first()).toContainText('bold text');
  await expect(page.locator('.markdown strong').first()).toBeVisible();
});

test('dashboard shows Pending badge after task submission', async ({ page }) => {
  const id = uniqueId('student');
  await signup(page, id, 'Badge Student', 'StudentPass123!');

  await page.goto(`/user/${id}/task/task1`);
  await page.fill('textarea[name="content"]', 'Badge test submission');
  await page.click('button[type="submit"]');

  await page.goto(`/user/${id}`);
  await expect(page.locator('body')).toContainText('Pending');
});

test('teacher can view student task and submit review', async ({ page }) => {
  const studentId = uniqueId('student');
  const studentPage = page;

  // Student signs up and submits a task
  await signup(studentPage, studentId, 'Reviewable Student', 'StudentPass123!');
  await studentPage.goto(`/user/${studentId}/task/task1`);
  await studentPage.fill('textarea[name="content"]', 'Please review my work');
  await studentPage.click('button[type="submit"]');

  // Teacher signs in (second browser context)
  const teacherContext = await page.context().browser()!.newContext();
  const teacherPage = await teacherContext.newPage();

  await teacherPage.goto('/');
  await teacherPage.fill('#signin-id', TEACHER_ID);
  await teacherPage.fill('#signin-password', TEACHER_PASSWORD);
  await teacherPage.click('form[hx-post="/signin"] button[type="submit"]');
  await teacherPage.waitForURL(new RegExp(`/user/${TEACHER_ID}`));

  // Teacher views student task
  await teacherPage.goto(`/user/${studentId}/task/task1`);
  await expect(teacherPage.locator('body')).toContainText('Please review my work');

  // Teacher submits review
  await teacherPage.fill('textarea[name="content"]', 'Good work, keep it up!');
  await teacherPage.click('button[type="submit"]');

  await teacherPage.waitForURL(`/user/${studentId}/task/task1`);
  await expect(teacherPage.locator('body')).toContainText('Good work, keep it up!');
  await expect(teacherPage.locator('body')).toContainText('Review');

  await teacherContext.close();
});
