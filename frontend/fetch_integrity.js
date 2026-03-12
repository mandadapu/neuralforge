const https = require('https');
let data = '';
https.get('https://registry.npmjs.org/minimatch/10.0.1', function(r) {
  r.on('data', function(c) { data += c; });
  r.on('end', function() {
    const j = JSON.parse(data);
    console.log(j.dist.integrity);
  });
});
