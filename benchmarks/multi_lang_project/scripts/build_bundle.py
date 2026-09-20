import os

os.makedirs('dist', exist_ok=True)
with open('dist/bundle.js', 'w') as f:
    f.write('// bundled web assets\nconsole.log("ready");\n')
print('Web bundle built successfully')
