const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');
const routesDir = path.join(root, 'routes');
const controllersDir = path.join(root, 'controllers');
const openAPIPath = path.join(root, 'openapi', 'openapi.json');

const HTTP_METHODS = new Set(['get', 'post', 'put', 'delete', 'patch', 'options', 'head']);
const mediaTypeMap = {
  json: 'application/json',
  xml: 'application/xml',
  plain: 'text/plain',
  html: 'text/html',
  multipart: 'multipart/form-data',
  formdata: 'multipart/form-data',
  form: 'application/x-www-form-urlencoded',
  urlencoded: 'application/x-www-form-urlencoded',
  octetstream: 'application/octet-stream',
};

const routeSeedGroups = {
  'bank_reconciliation_routes.go': { router: '' },
  'cashbank_integrated_routes.go': { router: '/api/v1' },
  'cashbank_ssot_routes.go': { v1: '/api/v1' },
  'contact_history_routes.go': { router: '' },
  'enhanced_report_routes.go': { router: '/api/v1', v1: '/api/v1' },
  'invoice_routes_example.go': { protected: '/api/v1' },
  'payment_routes.go': { router: '/api/v1' },
  'receipt_routes.go': { protected: '/api/v1' },
  'routes.go': { r: '' },
  'settings_routes.go': { protected: '/api/v1' },
  'ssot_payment_routes.go': { router: '/api/v1' },
  'ssot_report_routes.go': { router: '/api/v1', v1: '/api/v1' },
  'ultra_fast_payment_routes.go': { router: '' },
};

const quotedRe = /^\s*"([^"]*)"\s*$/;
const varAssignRe = /\b(\w+)\s*:?=\s*"([^"]*)"/;
const groupAssignRe = /\b(\w+)\s*:?=\s*(\w+)\.Group\(([^)]*)\)/;
const methodCallRe = /\b(\w+)\.(GET|POST|PUT|DELETE|PATCH|OPTIONS|HEAD)\((.+)\)\s*$/;
const constructorAssignRe = /\b(\w+)\s*:?=\s*(?:controllers|handlers)\.New([A-Za-z0-9_]+)\(/;
const controllerParamRe = /\b(\w+)\s+\*(?:controllers|handlers)\.([A-Za-z0-9_]+)/g;
const methodDeclRe = /^func\s*\(([^)]*)\)\s*([A-Za-z0-9_]+)\s*\(/;
const funcDeclRe = /^func\s+([A-Za-z0-9_]+)\s*\(/;

function deepClone(value) {
  return JSON.parse(JSON.stringify(value));
}

function normalizePath(routePath) {
  let normalized = (routePath || '')
    .replace(/:([A-Za-z_][A-Za-z0-9_]*)/g, '{$1}')
    .replace(/\/+/g, '/');

  if (!normalized.startsWith('/')) {
    normalized = `/${normalized}`;
  }

  if (normalized !== '/' && normalized.endsWith('/')) {
    normalized = normalized.slice(0, -1);
  }

  return normalized;
}

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

  return normalizePath(fullPath);
}

function resolvePathExpression(expr, stringVars) {
  const trimmed = expr.trim();
  const quoted = trimmed.match(quotedRe);
  if (quoted) {
    return quoted[1];
  }
  return stringVars[trimmed];
}

function listRouteFiles() {
  return fs
    .readdirSync(routesDir)
    .filter((file) => file.endsWith('.go'))
    .sort();
}

function listControllerFiles() {
  return fs
    .readdirSync(controllersDir)
    .filter((file) => file.endsWith('.go'))
    .sort();
}

function deriveOperationId(method, routePath) {
  const cleaned = normalizePath(routePath)
    .replace(/[{}]/g, '')
    .replace(/[^A-Za-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');
  return `${method}_${cleaned || 'root'}`.toLowerCase();
}

function deriveTag(routePath) {
  const normalized = normalizePath(routePath);
  const parts = normalized.split('/').filter(Boolean);
  if (parts.length === 0) {
    return 'General';
  }

  if (parts[0] === 'api') {
    if (parts[1] === 'v1' && parts[2]) {
      return parts[2]
        .split('-')
        .map((part) => (part ? `${part[0].toUpperCase()}${part.slice(1)}` : part))
        .join(' ');
    }
    if (parts[1]) {
      return parts[1]
        .split('-')
        .map((part) => (part ? `${part[0].toUpperCase()}${part.slice(1)}` : part))
        .join(' ');
    }
  }

  return parts[0]
    .split('-')
    .map((part) => (part ? `${part[0].toUpperCase()}${part.slice(1)}` : part))
    .join(' ');
}

function shouldDocumentRoute(routePath) {
  const normalized = normalizePath(routePath);
  return (
    normalized.startsWith('/api/') ||
    normalized.startsWith('/auth/') ||
    normalized === '/profile'
  );
}

function parseRouteHandlerRef(args) {
  if (args.includes('func(')) {
    return null;
  }

  const trimmed = args.trim();
  const match = trimmed.match(/([A-Za-z_][A-Za-z0-9_\.]*)\s*$/);
  if (!match) {
    return null;
  }
  return match[1];
}

function collectControllerVarTypes(contents) {
  const controllerVars = {};

  for (const match of contents.matchAll(controllerParamRe)) {
    controllerVars[match[1]] = match[2];
  }

  for (const match of contents.matchAll(new RegExp(constructorAssignRe, 'g'))) {
    controllerVars[match[1]] = match[2];
  }

  return controllerVars;
}

function collectRegisteredRoutes() {
  const routeMap = new Map();

  for (const fileName of listRouteFiles()) {
    const fullPath = path.join(routesDir, fileName);
    const contents = fs.readFileSync(fullPath, 'utf8');
    const lines = contents.split(/\r?\n/);
    const groupVars = { ...(routeSeedGroups[fileName] || {}) };
    const stringVars = {};
    const controllerVars = collectControllerVarTypes(contents);

    lines.forEach((rawLine, index) => {
      const line = rawLine.trim();
      if (!line || line.startsWith('//')) {
        return;
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
        return;
      }

      const methodMatch = line.match(methodCallRe);
      if (!methodMatch) {
        return;
      }

      const [, groupVar, methodName, args] = methodMatch;
      const base = groupVars[groupVar];
      if (base == null) {
        return;
      }

      const firstComma = args.indexOf(',');
      const pathExpr = firstComma === -1 ? args : args.slice(0, firstComma);
      const segment = resolvePathExpression(pathExpr, stringVars);
      if (segment == null) {
        return;
      }

      const routePath = normalizePath(joinPath(base, segment));
      const method = methodName.toLowerCase();
      const handlerRef = parseRouteHandlerRef(args);

      let handlerKey = null;
      if (handlerRef && handlerRef.includes('.')) {
        const [varName, methodRef] = handlerRef.split('.', 2);
        if (controllerVars[varName]) {
          handlerKey = `${controllerVars[varName]}.${methodRef}`;
        }
      } else if (handlerRef) {
        handlerKey = handlerRef;
      }

      const key = `${method.toUpperCase()} ${routePath}`;
      if (!routeMap.has(key)) {
        routeMap.set(key, {
          key,
          path: routePath,
          method,
          handlerRef,
          handlerKey,
          sourceFile: fileName,
          sourceLine: index + 1,
        });
      }
    });
  }

  return Array.from(routeMap.values()).filter((route) => shouldDocumentRoute(route.path));
}

function mapMediaType(token) {
  const normalized = token.trim().toLowerCase();
  if (!normalized) {
    return null;
  }
  return mediaTypeMap[normalized] || normalized;
}

function parsePrimitiveSchema(typeName) {
  const lower = typeName.toLowerCase();
  if (lower === 'string') {
    return { type: 'string' };
  }
  if (['int', 'int32', 'uint', 'uint32'].includes(lower)) {
    return { type: 'integer', format: 'int32' };
  }
  if (['int64', 'uint64'].includes(lower)) {
    return { type: 'integer', format: 'int64' };
  }
  if (['float', 'float32'].includes(lower)) {
    return { type: 'number', format: 'float' };
  }
  if (['double', 'float64'].includes(lower)) {
    return { type: 'number', format: 'double' };
  }
  if (['bool', 'boolean'].includes(lower)) {
    return { type: 'boolean' };
  }
  if (lower === 'file') {
    return { type: 'file' };
  }
  return null;
}

function schemaFromType(typeName) {
  const trimmed = typeName.trim();
  if (!trimmed) {
    return null;
  }

  if (trimmed.startsWith('[]')) {
    return {
      type: 'array',
      items: schemaFromType(trimmed.slice(2)) || { type: 'string' },
    };
  }

  if (trimmed === 'interface{}' || trimmed === 'gin.H') {
    return { type: 'object', additionalProperties: true };
  }

  if (/^map\[[^\]]+\]/.test(trimmed)) {
    const valueType = trimmed.replace(/^map\[[^\]]+\]/, '');
    return {
      type: 'object',
      additionalProperties: schemaFromType(valueType) || true,
    };
  }

  const primitive = parsePrimitiveSchema(trimmed);
  if (primitive) {
    return primitive;
  }

  return { $ref: `#/definitions/${trimmed}` };
}

function applyCommonParamExtras(param, extras) {
  if (!extras) {
    return;
  }

  const defaultMatch = extras.match(/default\(([^)]+)\)/i);
  if (defaultMatch) {
    param.default = defaultMatch[1];
  }

  const exampleMatch = extras.match(/example\(([^)]+)\)/i);
  if (exampleMatch) {
    param.example = exampleMatch[1];
  }
}

function parseParamAnnotation(payload) {
  const fullMatch = payload.match(/^(\S+)\s+(\S+)\s+(.+?)\s+(true|false)\s+"([^"]*)"(.*)$/);
  const fallbackMatch =
    fullMatch || payload.match(/^(\S+)\s+(\S+)\s+(.+?)\s+(true|false)(.*)$/);

  if (!fallbackMatch) {
    return null;
  }

  const [, name, location, typeName, requiredRaw] = fallbackMatch;
  const description = fullMatch ? fullMatch[5] : '';
  const extras = fullMatch ? fullMatch[6] : fallbackMatch[5];
  const required = requiredRaw === 'true';

  const param = {
    name,
    in: location,
    description,
    required,
  };

  if (location === 'body') {
    param.schema = schemaFromType(typeName) || { type: 'object' };
  } else {
    const schema = schemaFromType(typeName);
    if (schema) {
      if (schema.$ref) {
        param.type = 'string';
      } else if (schema.type === 'array') {
        param.type = 'array';
        param.items = schema.items || { type: 'string' };
      } else if (schema.type === 'file') {
        param.type = 'file';
      } else {
        param.type = schema.type;
        if (schema.format) {
          param.format = schema.format;
        }
      }
    } else {
      param.type = 'string';
    }
  }

  applyCommonParamExtras(param, extras);
  return param;
}

function parseResponseAnnotation(payload) {
  const match = payload.match(/^(\d+)\s+\{([^}]+)\}\s+(\S+)(?:\s+"([^"]*)")?/);
  if (!match) {
    return null;
  }

  const [, code, schemaKind, typeName, description] = match;
  const response = {
    description: description || 'Success',
  };

  const lowerKind = schemaKind.toLowerCase();
  if (lowerKind === 'object' || lowerKind === 'array') {
    response.schema = schemaFromType(lowerKind === 'array' && !typeName.startsWith('[]') ? `[]${typeName}` : typeName);
  } else if (lowerKind === 'string' || lowerKind === 'integer' || lowerKind === 'boolean' || lowerKind === 'number') {
    response.schema = { type: lowerKind };
  }

  return { code, response };
}

function parseRouterAnnotation(payload) {
  const match = payload.match(/^(\S+)\s+\[([a-zA-Z]+)\]$/);
  if (!match) {
    return null;
  }

  return {
    path: normalizePath(match[1]),
    method: match[2].toLowerCase(),
  };
}

function parseAnnotationBlock(commentLines) {
  const annotation = {
    summary: '',
    descriptionParts: [],
    tags: [],
    consumes: [],
    produces: [],
    parameters: [],
    responses: {},
    security: [],
    router: null,
    deprecated: false,
  };

  for (const rawLine of commentLines) {
    const line = rawLine.replace(/^\s*\/\/\s?/, '').trim();
    if (!line.startsWith('@')) {
      continue;
    }

    const spaceIndex = line.indexOf(' ');
    const key = (spaceIndex === -1 ? line.slice(1) : line.slice(1, spaceIndex)).trim();
    const payload = (spaceIndex === -1 ? '' : line.slice(spaceIndex + 1)).trim();

    switch (key.toLowerCase()) {
      case 'summary':
        annotation.summary = payload;
        break;
      case 'description':
        annotation.descriptionParts.push(payload);
        break;
      case 'tags':
        annotation.tags.push(
          ...payload
            .split(',')
            .map((tag) => tag.trim())
            .filter(Boolean)
        );
        break;
      case 'accept':
        annotation.consumes.push(
          ...payload
            .split(/[,\s]+/)
            .map(mapMediaType)
            .filter(Boolean)
        );
        break;
      case 'produce':
        annotation.produces.push(
          ...payload
            .split(/[,\s]+/)
            .map(mapMediaType)
            .filter(Boolean)
        );
        break;
      case 'param': {
        const param = parseParamAnnotation(payload);
        if (param) {
          annotation.parameters.push(param);
        }
        break;
      }
      case 'success':
      case 'failure':
      case 'response': {
        const parsed = parseResponseAnnotation(payload);
        if (parsed) {
          annotation.responses[parsed.code] = parsed.response;
        }
        break;
      }
      case 'security': {
        const securityName = payload === 'Bearer' ? 'BearerAuth' : payload;
        annotation.security.push({ [securityName]: [] });
        break;
      }
      case 'router':
        annotation.router = parseRouterAnnotation(payload);
        break;
      case 'deprecated':
        annotation.deprecated = true;
        break;
      default:
        break;
    }
  }

  if (annotation.descriptionParts.length > 0) {
    annotation.description = annotation.descriptionParts.join(' ').trim();
  }
  delete annotation.descriptionParts;

  return annotation;
}

function parseAnnotatedOperations() {
  const docsByHandler = new Map();
  const docsByRouter = new Map();

  for (const fileName of listControllerFiles()) {
    const fullPath = path.join(controllersDir, fileName);
    const lines = fs.readFileSync(fullPath, 'utf8').split(/\r?\n/);
    let pendingComments = [];

    lines.forEach((rawLine) => {
      const line = rawLine.trim();

      if (line.startsWith('//')) {
        pendingComments.push(rawLine);
        return;
      }

      const methodMatch = line.match(methodDeclRe);
      const funcMatch = methodMatch ? null : line.match(funcDeclRe);

      if (methodMatch || funcMatch) {
        const annotation = parseAnnotationBlock(pendingComments);
        if (annotation.router || annotation.summary || Object.keys(annotation.responses).length > 0) {
          let key;
          if (methodMatch) {
            const receiverType = methodMatch[1].match(/\*?([A-Za-z0-9_]+)/)[1];
            const methodName = methodMatch[2];
            key = `${receiverType}.${methodName}`;
          } else {
            key = funcMatch[1];
          }

          docsByHandler.set(key, annotation);

          if (annotation.router && HTTP_METHODS.has(annotation.router.method)) {
            docsByRouter.set(
              `${annotation.router.method.toUpperCase()} ${annotation.router.path}`,
              annotation
            );
          }
        }

        pendingComments = [];
        return;
      }

      if (line === '') {
        pendingComments = [];
      }
    });
  }

  return { docsByHandler, docsByRouter };
}

function loadBaseSpec() {
  if (!fs.existsSync(openAPIPath)) {
    throw new Error(`OpenAPI source not found: ${openAPIPath}`);
  }
  return JSON.parse(fs.readFileSync(openAPIPath, 'utf8'));
}

function buildOperationFromAnnotation(annotation, route) {
  const operation = {};

  if (annotation.summary) {
    operation.summary = annotation.summary;
  }
  if (annotation.description) {
    operation.description = annotation.description;
  }
  if (annotation.tags && annotation.tags.length > 0) {
    operation.tags = Array.from(new Set(annotation.tags));
  }
  if (annotation.consumes && annotation.consumes.length > 0) {
    operation.consumes = Array.from(new Set(annotation.consumes));
  }
  if (annotation.produces && annotation.produces.length > 0) {
    operation.produces = Array.from(new Set(annotation.produces));
  }
  if (annotation.parameters && annotation.parameters.length > 0) {
    operation.parameters = deepClone(annotation.parameters);
  }
  if (annotation.responses && Object.keys(annotation.responses).length > 0) {
    operation.responses = deepClone(annotation.responses);
  }
  if (annotation.security && annotation.security.length > 0) {
    operation.security = deepClone(annotation.security);
  }
  if (annotation.deprecated) {
    operation.deprecated = true;
  }

  operation.operationId = deriveOperationId(route.method, route.path);
  return operation;
}

function ensurePathParameters(operation, routePath) {
  const paramNames = Array.from(routePath.matchAll(/\{([^}]+)\}/g)).map((match) => match[1]);
  if (paramNames.length === 0) {
    return;
  }

  if (!Array.isArray(operation.parameters)) {
    operation.parameters = [];
  }

  const existing = new Set(
    operation.parameters
      .filter((param) => param.in === 'path')
      .map((param) => param.name)
  );

  for (const name of paramNames) {
    if (existing.has(name)) {
      continue;
    }
    operation.parameters.push({
      name,
      in: 'path',
      required: true,
      type: 'string',
      description: `${name} path parameter`,
    });
  }
}

function createPlaceholderOperation(route) {
  const methodLabel = route.method.toUpperCase();
  const operation = {
    summary: `${methodLabel} ${route.path}`,
    description:
      `Auto-generated documentation placeholder for active route ${methodLabel} ${route.path}. ` +
      'Add Swagger annotations to the bound handler for full request/response metadata.',
    tags: [deriveTag(route.path)],
    operationId: deriveOperationId(route.method, route.path),
    responses: {
      200: {
        description: 'Success',
      },
    },
  };

  ensurePathParameters(operation, route.path);
  return operation;
}

function isPlaceholderOperation(operation) {
  return (
    operation &&
    typeof operation.description === 'string' &&
    operation.description.startsWith('Auto-generated documentation placeholder for active route ')
  );
}

function mergeOperation(primary, fallback) {
  if (!primary && !fallback) {
    return {};
  }
  if (!primary) {
    return deepClone(fallback);
  }
  if (!fallback) {
    return deepClone(primary);
  }

  const merged = deepClone(fallback);
  const preferred = deepClone(primary);

  for (const [key, value] of Object.entries(preferred)) {
    if (Array.isArray(value)) {
      merged[key] = value;
    } else if (value && typeof value === 'object' && !('$ref' in value)) {
      merged[key] = { ...(merged[key] || {}), ...value };
    } else {
      merged[key] = value;
    }
  }

  return merged;
}

function generateSpec() {
  const baseSpec = loadBaseSpec();
  const routes = collectRegisteredRoutes();
  const { docsByHandler, docsByRouter } = parseAnnotatedOperations();
  const nextSpec = deepClone(baseSpec);

  nextSpec.swagger = nextSpec.swagger || '2.0';
  nextSpec.basePath = '/';
  nextSpec.paths = {};

  const documentedRouteKeys = [];
  let annotationBackedOperations = 0;
  let placeholderOperations = 0;

  for (const route of routes) {
    const exactBaseOperationCandidate =
      baseSpec.paths &&
      baseSpec.paths[route.path] &&
      baseSpec.paths[route.path][route.method]
        ? baseSpec.paths[route.path][route.method]
        : null;
    const exactBaseOperation = isPlaceholderOperation(exactBaseOperationCandidate)
      ? null
      : exactBaseOperationCandidate;

    const annotation =
      (route.handlerKey && docsByHandler.get(route.handlerKey)) ||
      docsByRouter.get(`${route.method.toUpperCase()} ${route.path}`) ||
      null;

    const annotationBaseCandidate =
      annotation &&
      annotation.router &&
      baseSpec.paths &&
      baseSpec.paths[annotation.router.path] &&
      baseSpec.paths[annotation.router.path][route.method]
        ? baseSpec.paths[annotation.router.path][route.method]
        : null;
    const annotationBase = isPlaceholderOperation(annotationBaseCandidate)
      ? null
      : annotationBaseCandidate;

    let operation;
    if (annotation) {
      operation = mergeOperation(buildOperationFromAnnotation(annotation, route), annotationBase || exactBaseOperation);
      annotationBackedOperations += 1;
    } else if (exactBaseOperation) {
      operation = deepClone(exactBaseOperation);
    } else {
      operation = createPlaceholderOperation(route);
      placeholderOperations += 1;
    }

    if (!operation.tags || operation.tags.length === 0) {
      operation.tags = [deriveTag(route.path)];
    }
    if (!operation.operationId) {
      operation.operationId = deriveOperationId(route.method, route.path);
    }
    if (!operation.responses || Object.keys(operation.responses).length === 0) {
      operation.responses = { 200: { description: 'Success' } };
    }

    ensurePathParameters(operation, route.path);

    if (!nextSpec.paths[route.path]) {
      nextSpec.paths[route.path] = {};
    }
    nextSpec.paths[route.path][route.method] = operation;
    documentedRouteKeys.push(`${route.method.toUpperCase()} ${route.path}`);
  }

  const usedTags = new Set();
  for (const methods of Object.values(nextSpec.paths)) {
    for (const operation of Object.values(methods)) {
      for (const tag of operation.tags || []) {
        usedTags.add(tag);
      }
    }
  }

  const existingTags = new Map(
    (baseSpec.tags || []).map((tag) => [tag.name, tag])
  );
  nextSpec.tags = Array.from(usedTags)
    .sort((a, b) => a.localeCompare(b))
    .map((tagName) => existingTags.get(tagName) || { name: tagName });

  if (nextSpec.info && typeof nextSpec.info.description === 'string') {
    const marker = 'This document is generated from active Gin routes and controller Swagger annotations.';
    if (!nextSpec.info.description.includes(marker)) {
      nextSpec.info.description = `${nextSpec.info.description}\n\n${marker}`;
    }
  }

  return {
    spec: nextSpec,
    stats: {
      routeCount: routes.length,
      pathCount: Object.keys(nextSpec.paths).length,
      operationCount: documentedRouteKeys.length,
      annotationBackedOperations,
      placeholderOperations,
    },
    routes,
  };
}

function readSpec() {
  if (!fs.existsSync(openAPIPath)) {
    throw new Error(`OpenAPI source not found: ${openAPIPath}`);
  }
  return JSON.parse(fs.readFileSync(openAPIPath, 'utf8'));
}

function collectSpecOperations(spec) {
  const map = new Map();
  for (const [routePath, methods] of Object.entries(spec.paths || {})) {
    for (const [method, operation] of Object.entries(methods)) {
      const lowered = method.toLowerCase();
      if (!HTTP_METHODS.has(lowered)) {
        continue;
      }
      map.set(`${lowered.toUpperCase()} ${normalizePath(routePath)}`, operation);
    }
  }
  return map;
}

module.exports = {
  collectRegisteredRoutes,
  collectSpecOperations,
  generateSpec,
  normalizePath,
  openAPIPath,
  readSpec,
  shouldDocumentRoute,
};
