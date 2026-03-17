import { Page } from '@playwright/test';

export const BASE_URL = 'http://localhost:8080';

// Teacher ID must match teacher_ids in the server config (default: conf/config.yaml)
export const TEACHER_ID = '123';
export const TEACHER_PASSWORD = 'TeacherPass123!';
export const TEACHER_USERNAME = 'E2E Teacher';

let _counter = Date.now();
export function uniqueId(prefix = 'user'): string {
  return `${prefix}_${_counter++}`;
}

/** Sign up and return a session cookie value. */
export async function signup(
  page: Page,
  id: string,
  username: string,
  password: string,
): Promise<string> {
  await page.goto('/');
  await page.fill('#signup-id', id);
  await page.fill('#signup-username', username);
  await page.fill('#signup-password', password);
  await page.click('form[hx-post="/signup"] button[type="submit"]');

  await page.waitForURL(new RegExp(`/user/${id}`));

  const cookies = await page.context().cookies();
  const sessionCookie = cookies.find(c => c.name === 'user_data');
  if (!sessionCookie) throw new Error('Session cookie not set after signup');
  return sessionCookie.value;
}

/** Sign in and return a session cookie value. */
export async function signin(
  page: Page,
  id: string,
  password: string,
): Promise<string> {
  await page.goto('/');
  await page.fill('#signin-id', id);
  await page.fill('#signin-password', password);
  await page.click('form[hx-post="/signin"] button[type="submit"]');

  await page.waitForURL(new RegExp(`/user/${id}`));

  const cookies = await page.context().cookies();
  const sessionCookie = cookies.find(c => c.name === 'user_data');
  if (!sessionCookie) throw new Error('Session cookie not set after signin');
  return sessionCookie.value;
}
