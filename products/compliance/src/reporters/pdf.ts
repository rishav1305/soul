/**
 * PDF report generation via Puppeteer.
 *
 * Puppeteer is an optional peer dependency. If it is not installed, a
 * helpful error message is thrown instructing the user how to install it.
 *
 * The dynamic import is wrapped so that TypeScript does not require
 * puppeteer type declarations at compile time.
 */

/* eslint-disable @typescript-eslint/no-explicit-any */

async function loadPuppeteer(): Promise<any> {
  try {
    // Dynamic import — puppeteer may not be installed.
    // The string indirection prevents tsc from resolving the module.
    const mod = 'puppeteer';
    return await import(/* webpackIgnore: true */ mod);
  } catch {
    throw new Error('PDF generation requires puppeteer. Run: npm i -g puppeteer');
  }
}

export async function generatePdf(html: string, outputPath: string): Promise<void> {
  const puppeteer = await loadPuppeteer();
  const launch = puppeteer.default?.launch ?? puppeteer.launch;
  const browser = await launch({ headless: true });
  const page = await browser.newPage();
  await page.setContent(html, { waitUntil: 'networkidle0' });
  await page.pdf({ path: outputPath, format: 'A4', printBackground: true });
  await browser.close();
}
