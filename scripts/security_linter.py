import sys
import os
import re

def lint(path):
    issues = []
    for root, _, files in os.walk(path):
        for f in files:
            if f.endswith('.go'):
                filepath = os.path.join(root, f)
                with open(filepath, 'r', encoding='utf-8') as file:
                    content = file.read()
                    
                    if 'packages.Config' in content and 'CGO_ENABLED=0' not in content:
                        issues.append(f"{filepath}: packages.Config found without CGO_ENABLED=0 mitigation.")
                    
                    if 'filepath.Abs' in content and ', _ :=' in content:
                        issues.append(f"{filepath}: filepath.Abs error is being ignored.")
                        
    if issues:
        print("FAIL: Security Linter found critical issues:")
        for issue in issues:
            print(f" - {issue}")
        sys.exit(1)
    else:
        print("PASS: No security issues found.")
        sys.exit(0)

if __name__ == '__main__':
    lint(sys.argv[1] if len(sys.argv) > 1 else '.')
