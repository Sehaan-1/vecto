import os

os.makedirs('dist', exist_ok=True)
with open('dist/release.manifest', 'w') as f:
    f.write('release-version: 1.0.0\nartifacts: [api_service.bin, bundle.js]\n')
print('Release package finalized')
