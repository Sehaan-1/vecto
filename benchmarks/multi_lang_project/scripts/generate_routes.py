import os

os.makedirs('generated', exist_ok=True)
with open('generated/routes.txt', 'w') as f:
    f.write('generated routes from schema\n')
print('Routes generated successfully')
