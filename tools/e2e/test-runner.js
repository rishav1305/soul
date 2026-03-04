const { chromium } = require('playwright');

// Usage: node test-runner.js <action> <url> [args...]
// Actions:
//   screenshot <url> [selector]           - take screenshot
//   check <url> <selector>                - check element exists, return text
//   eval <url> <js_expression>            - evaluate JS in page
//   assert <url> <json_assertions>        - run multiple assertions
//   dom <url> [selector]                  - structured DOM snapshot

var args = process.argv.slice(2);
var action, url, extra;

// Support --json mode: read args from a JSON file
if (args[0] === '--json') {
  var fs = require('fs');
  var jsonArgs = JSON.parse(fs.readFileSync(args[1], 'utf8'));
  action = jsonArgs.action;
  url = jsonArgs.url;
  extra = jsonArgs; // contains selector, assertions, etc.
} else {
  action = args[0];
  url = args[1];
  extra = {};
}

async function run() {
  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage'],
  });
  const page = await browser.newPage({ viewport: { width: 1920, height: 1080 } });

  try {
    await page.goto(url, { waitUntil: 'networkidle', timeout: 30000 });
    // Wait for React to hydrate
    await page.waitForSelector('#root > *', { timeout: 10000 }).catch(function() {});
    await page.waitForTimeout(1000);

    if (action === 'screenshot') {
      var selector = extra.selector || args[2];
      var path = '/tmp/soul-e2e-screenshot.png';
      if (selector) {
        var el = await page.$(selector);
        if (el) {
          await el.screenshot({ path: path });
        } else {
          console.log(JSON.stringify({ error: 'Element not found: ' + selector }));
          await browser.close();
          return;
        }
      } else {
        await page.screenshot({ path: path, fullPage: false });
      }
      var fs = require('fs');
      console.log(JSON.stringify({ path: path, size: fs.statSync(path).size }));

    } else if (action === 'check') {
      var selector = extra.selector || args[2];
      var elements = await page.$$(selector);
      var results = [];
      var slice = elements.slice(0, 20);
      for (var i = 0; i < slice.length; i++) {
        var el = slice[i];
        var text = '';
        try { text = await el.textContent(); } catch(e) {}
        var visible = false;
        try { visible = await el.isVisible(); } catch(e) {}
        var box = null;
        try { box = await el.boundingBox(); } catch(e) {}
        results.push({
          text: (text || '').trim().slice(0, 200),
          visible: visible,
          box: box,
        });
      }
      console.log(JSON.stringify({ selector: selector, count: elements.length, elements: results }));

    } else if (action === 'eval') {
      var expr = extra.expression || args[2];
      var result = await page.evaluate(expr);
      console.log(JSON.stringify({ result: result }));

    } else if (action === 'assert') {
      var assertions = extra.assertions || JSON.parse(args[2]);
      var results = [];
      for (var i = 0; i < assertions.length; i++) {
        var a = assertions[i];
        var pass = false;
        var detail = '';
        try {
          if (a.type === 'exists') {
            var el = await page.$(a.selector);
            pass = !!el;
            if (el) {
              var t = await el.textContent();
              detail = (t || '').trim().slice(0, 100);
            }
          } else if (a.type === 'visible') {
            var el = await page.$(a.selector);
            pass = el ? await el.isVisible() : false;
          } else if (a.type === 'text_contains') {
            var el = await page.$(a.selector);
            if (el) {
              var text = await el.textContent();
              pass = (text || '').includes(a.expected);
              detail = (text || '').trim().slice(0, 100);
            }
          } else if (a.type === 'count') {
            var els = await page.$$(a.selector);
            pass = a.min ? els.length >= a.min : els.length === a.expected;
            detail = 'found ' + els.length;
          } else if (a.type === 'eval') {
            var result = await page.evaluate(a.expression);
            pass = !!result;
            detail = JSON.stringify(result).slice(0, 100);
          }
        } catch (e) {
          detail = e.message.slice(0, 100);
        }
        results.push({ type: a.type, selector: a.selector, pass: pass, detail: detail });
      }
      var allPass = results.every(function(r) { return r.pass; });
      console.log(JSON.stringify({ allPass: allPass, results: results }));

    } else if (action === 'dom') {
      var selector = extra.selector || args[2] || 'body';
      var snapshot = await page.evaluate(function(sel) {
        function walk(el, depth) {
          if (depth > 5) return '';
          var tag = (el.tagName || '').toLowerCase();
          var id = el.id ? '#' + el.id : '';
          var cls = '';
          if (el.className && typeof el.className === 'string') {
            var parts = el.className.split(/\s+/).filter(Boolean).slice(0, 3);
            if (parts.length) cls = '.' + parts.join('.');
          }
          var text = '';
          if (el.childNodes.length === 1 && el.childNodes[0].nodeType === 3) {
            var t = el.textContent.trim().slice(0, 80);
            if (t) text = ' "' + t + '"';
          }
          var indent = '';
          for (var i = 0; i < depth; i++) indent += '  ';
          var result = indent + '<' + tag + id + cls + text + '>\n';
          for (var c = 0; c < el.children.length; c++) {
            result += walk(el.children[c], depth + 1);
          }
          return result;
        }
        var root = document.querySelector(sel);
        return root ? walk(root, 0) : 'Element not found: ' + sel;
      }, selector);
      // Output directly, capped at 5000 chars
      var out = snapshot.slice(0, 5000);
      console.log(out);

    } else {
      console.log(JSON.stringify({ error: 'Unknown action: ' + action }));
    }
  } catch (e) {
    console.log(JSON.stringify({ error: e.message }));
  } finally {
    await browser.close();
  }
}

run();
