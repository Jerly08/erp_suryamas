const {
  collectRegisteredRoutes,
  collectSpecOperations,
  readSpec,
} = require('./openapi_pipeline_lib');

function main() {
  const routes = collectRegisteredRoutes();
  const spec = readSpec();
  const specOperations = collectSpecOperations(spec);
  const routeKeys = new Set(routes.map((route) => route.key));

  const missingInSpec = routes
    .filter((route) => !specOperations.has(route.key))
    .map((route) => `${route.key} (${route.sourceFile}:${route.sourceLine})`);

  const staleInSpec = Array.from(specOperations.keys()).filter((key) => !routeKeys.has(key));

  if (missingInSpec.length > 0 || staleInSpec.length > 0) {
    console.error('OpenAPI verification failed.');

    if (missingInSpec.length > 0) {
      console.error('\nRoutes missing from spec:');
      for (const route of missingInSpec) {
        console.error(`- ${route}`);
      }
    }

    if (staleInSpec.length > 0) {
      console.error('\nSpec entries without active route:');
      for (const route of staleInSpec) {
        console.error(`- ${route}`);
      }
    }

    process.exit(1);
  }

  console.log('OpenAPI verification passed.');
  console.log(`Documented operations: ${specOperations.size}`);
  console.log(`Active routes checked: ${routes.length}`);
}

main();
