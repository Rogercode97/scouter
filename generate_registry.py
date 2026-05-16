import os
import glob
import yaml
import re

# Use relative paths based on script location
PROJECT_ROOT = os.path.dirname(os.path.abspath(__file__))
SKILLS_DIR = os.path.join(PROJECT_ROOT, "skills")
ATL_DIR = os.path.join(PROJECT_ROOT, ".atl")

def parse_frontmatter(content):
    match = re.match(r'^---\n(.*?)\n---', content, re.DOTALL)
    if match:
        try:
            return yaml.safe_load(match.group(1))
        except:
            return {}
    return {}

def extract_trigger(description):
    if not description: return ""
    match = re.search(r'Trigger:\s*(.*)', description, re.IGNORECASE)
    if match:
        return match.group(1).strip()
    return description.strip()

def extract_compact_rules(content):
    # Try to find Rules, Critical Patterns, Mandates, etc.
    rules = []
    in_rules_section = False
    for line in content.split('\n'):
        if re.match(r'^##\s+(Compact Rules|Rules|Critical Patterns|Mandates|Core Principles|Guidelines|What to Do|Instructions)', line, re.IGNORECASE):
            in_rules_section = True
            continue
        elif re.match(r'^##\s+', line) and in_rules_section:
            break
        
        if in_rules_section and line.strip().startswith('-'):
            rules.append(line.strip())
            if len(rules) >= 15:
                break
                
    if not rules:
        # Fallback: just grab the first few bullet points in the file
        for line in content.split('\n'):
            if line.strip().startswith('-'):
                rules.append(line.strip())
                if len(rules) >= 10:
                    break
                    
    # Ensure we have at least something, or a placeholder
    if not rules:
        rules = ["- Follow standard practices for this skill."]
        
    return rules[:15]

skills = []
for filepath in glob.glob(os.path.join(SKILLS_DIR, "**/SKILL.md"), recursive=True):
    if "sdd-" in filepath or "_shared" in filepath or "skill-registry" in filepath:
        continue
        
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()
        
    fm = parse_frontmatter(content)
    name = fm.get('name', os.path.basename(os.path.dirname(filepath)))
    description = fm.get('description', '')
    trigger = extract_trigger(description)
    rules = extract_compact_rules(content)
    
    skills.append({
        'name': name,
        'trigger': trigger,
        'path': filepath,
        'rules': rules
    })

# Sort skills by name
skills.sort(key=lambda x: x['name'])

# Find conventions
conventions = []
convention_files = ['AGENTS.md', 'agents.md', 'CLAUDE.md', '.cursorrules', 'GEMINI.md', 'copilot-instructions.md']
for cf in convention_files:
    cf_path = os.path.join(PROJECT_ROOT, cf)
    if os.path.exists(cf_path):
        conventions.append({
            'file': cf,
            'path': cf_path,
            'notes': 'Standalone file'
        })
        # If it's an index file, we should extract referenced paths
        if cf.lower() == 'agents.md':
            conventions[-1]['notes'] = 'Index - references files below'
            with open(cf_path, 'r', encoding='utf-8') as f:
                content = f.read()
                # Extract paths like ./something.md or just something.md
                paths = re.findall(r'](.*?\.md)', content)
                for p in paths:
                    full_p = os.path.normpath(os.path.join(PROJECT_ROOT, p))
                    conventions.append({
                        'file': os.path.basename(p),
                        'path': full_p,
                        'notes': f'Referenced by {cf}'
                    })

# Generate markdown
md = []
md.append("# Skill Registry\n")
md.append("**Delegator use only.** Any agent that launches sub-agents reads this registry to resolve compact rules, then injects them directly into sub-agent prompts. Sub-agents do NOT read this registry or individual SKILL.md files.\n")
md.append("See `_shared/skill-resolver.md` for the full resolution protocol.\n")

md.append("## User Skills\n")
md.append("| Trigger | Skill | Path |")
md.append("|---------|-------|------|")
for s in skills:
    md.append(f"| {s['trigger']} | {s['name']} | {s['path']} |")

md.append("\n## Compact Rules\n")
md.append("Pre-digested rules per skill. Delegators copy matching blocks into sub-agent prompts as `## Project Standards (auto-resolved)`.\n")

for s in skills:
    md.append(f"### {s['name']}")
    for r in s['rules']:
        md.append(r)
    md.append("")

md.append("## Project Conventions\n")
md.append("| File | Path | Notes |")
md.append("|------|------|-------|")
for c in conventions:
    md.append(f"| {c['file']} | {c['path']} | {c['notes']} |")

md.append("\nRead the convention files listed above for project-specific patterns and rules. All referenced paths have been extracted — no need to read index files to discover more.")

os.makedirs(ATL_DIR, exist_ok=True)
with open(os.path.join(ATL_DIR, "skill-registry.md"), 'w', encoding='utf-8') as f:
    f.write('\n'.join(md))

print("Registry generated successfully.")
