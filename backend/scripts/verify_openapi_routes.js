const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');
const sourcePath = path.join(root, 'openapi', 'openapi.json');

const routeFiles = [
  ['routes/routes.go', { r: '' }],
  ['routes/payment_routes.go', { router: '/api/v1' }],
  ['routes/settings_routes.go', { protected: '/api/v1' }],
  ['routes/receipt_routes.go', { protected: '/api/v1' }],
  ['routes/cashbank_integrated_routes.go', { router: '/api/v1' }],
  ['routes/cashbank_ssot_routes.go', { v1: '/api/v1' }],
  ['routes/ssot_payment_routes.go', { router: '/api/v1' }],
  ['routes/ssot_report_routes.go', { router: '/api/v1', v1: '/api/v1' }],
  ['routes/contact_history_routes.go', { router: '' }],
  ['routes/enhanced_report_routes.go', { router: '/api/v1', v1: '/api/v1' }],
  ['routes/ultra_fast_payment_routes.go', { router: '' }],
  ['routes/invoice_routes_example.go', { protected: '/api/v1' }],
];

const quotedRe = /^\s*"([^"]*)"\s*$/;
const varAssignRe = /\b(\w+)\s*:?\=\s*"([^"]*)"/;
const groupAssignRe = /\b(\w+)\s*:?\=\s*(\w+)\.Group\(([^)]*)\)/;
const methodRe = /\b(\w+)\.(GET|POST|PUT|DELETE|PATCH|OPTIONS|HEAD)\(([^,]+)/;

function joinPath(base, segment) {
  if (segment == null) {
    return null;
  }
  if (segment === '') {
    return base || '';
  }

  let fullPath;
  if (!base) {
    fullPath = segment;
  } else if (base.endsWith('/') && segment.startsWith('/')) {
    fullPath = base.slice(0, -1) + segment;
  } else if (!base.endsWith('/') && !segment.startsWith('/')) {
    fullPath = `${base}/${segment}`;
  } else {
    fullPath = base + segment;
  }

  fullPath = fullPath.replace(/\/+/g, '/');
  if (fullPath !== '/' && fullPath.endsWith('/')) {
    fullPath = fullPath.slice(0, -1);
  }
  return fullPath;
}

function normalizePath(routePath) {
  let normalized = (routePath || '')
    .replace(/:([A-Za-z_][A-Za-z0-9_]*)/g, '{$1}')
    .replace(/\/+/g, '/');

  if (normalized !== '/' && normalized.endsWith('/')) {
    normalized = normalized.slice(0, -1);
  }
  return normalized;
}

function resolvePathExpression(expr, stringVars) {
  const trimmed = expr.trim();
  const quoted = trimmed.match(quotedRe);
  if (quoted) {
    return quoted[1];
  }
  return stringVars[trimmed];
}

function addOperation(map, routePath, method) {
  if (!map.has(routePath)) {
    map.set(routePath, new Set());
  }
  map.get(routePath).add(method);
}

function collectRegisteredRoutes() {
  const routes = new Map();

  for (const [relativeFile, seedGroups] of routeFiles) {
    const fullPath = path.join(root, relativeFile);
    const lines = fs.readFileSync(fullPath, 'utf8').split(/\r?\n/);
    const groupVars = { ...seedGroups };
    const stringVars = {};

    for (const rawLine of lines) {
      const line = rawLine.trim();
      if (!line || line.startsWith('//')) {
        continue;
      }

      const varMatch = line.match(varAssignRe);
      if (varMatch && !line.includes('.Group(')) {
        stringVars[varMatch[1]] = varMatch[2];
      }

      const groupMatch = line.match(groupAssignRe);
      if (groupMatch) {
        const [, child, parent, expr] = groupMatch;
        const base = groupVars[parent];
        const segment = resolvePathExpression(expr, stringVars);
        if (base != null && segment != null) {
          groupVars[child] = joinPath(base, segment);
        }
        continue;
      }

      const methodMatch = line.match(methodRe);
      if (!methodMatch) {
        continue;
      }

      const [, groupVar, method, expr] = methodMatch;
      const base = groupVars[groupVar];
      const segment = resolvePathExpression(expr, stringVars);
      if (base == null || segment == null) {
        continue;
      }

      const routePath = normalizePath(joinPath(base, segment));
      addOperation(routes, routePath, method.toLowerCase());
    }
  }

  return routes;
}

function loadOpenAPI() {
  if (!fs.existsSync(sourcePath)) {
    throw new Error(`OpenAPI source not found: ${sourcePath}`);
  }
  return JSON.parse(fs.readFileSync(sourcePath, 'utf8'));
}

function main() {
  const codeRoutes = collectRegisteredRoutes();
  const openapi = loadOpenAPI();
  const specPaths = new Map();

  for (const [routePath, methods] of Object.entries(openapi.paths || {})) {
    for (const method of Object.keys(methods)) {
      const lowered = method.toLowerCase();
      if (!['get', 'post', 'put', 'delete', 'patch', 'options', 'head'].includes(lowered)) {
        continue;
      }
      addOperation(specPaths, normalizePath(routePath), lowered);
    }
  }

  const mismatches = [];
  for (const [routePath, methods] of specPaths.entries()) {
    if (!codeRoutes.has(routePath)) {
      mismatches.push(`${routePath} -> missing route`);
      continue;
    }

    for (const method of methods) {
      if (!codeRoutes.get(routePath).has(method)) {
        mismatches.push(`${method.toUpperCase()} ${routePath} -> method not registered`);
      }
    }
  }

  if (mismatches.length > 0) {
    console.error('OpenAPI verification failed.');
    for (const mismatch of mismatches) {
      console.error(`- ${mismatch}`);
    }
    process.exit(1);
  }

  console.log('OpenAPI verification passed.');
  console.log(`Documented paths: ${specPaths.size}`);
}

main();
