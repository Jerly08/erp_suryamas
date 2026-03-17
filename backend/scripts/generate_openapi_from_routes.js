const fs = require('fs');
const path = require('path');

const { generateSpec, openAPIPath } = require('./openapi_pipeline_lib');

function main() {
  const { spec, stats } = generateSpec();

  fs.mkdirSync(path.dirname(openAPIPath), { recursive: true });
  fs.writeFileSync(openAPIPath, `${JSON.stringify(spec, null, 2)}\n`, 'utf8');

  console.log(`Generated ${openAPIPath}`);
  console.log(`Documented paths: ${stats.pathCount}`);
  console.log(`Documented operations: ${stats.operationCount}`);
  console.log(`Operations backed by Swagger annotations: ${stats.annotationBackedOperations}`);
  console.log(`Placeholder operations (active route without annotations): ${stats.placeholderOperations}`);
}

main();
