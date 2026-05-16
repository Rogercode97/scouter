import os
import glob
import re
import yaml

GEMINI_SKILLS_DIR = os.path.expanduser("~/.gemini/skills")
PROJECT_ROOT = os.path.dirname(os.path.abspath(__file__))
SOVEREIGN_SKILLS_DIR = os.path.join(PROJECT_ROOT, "skills")

def parse_frontmatter(content):
    match = re.match(r'^---\n(.*?)\n---', content, re.DOTALL)
    if match:
        try:
            return yaml.safe_load(match.group(1))
        except:
            return {}
    return {}

def fix_symlinks():
    if not os.path.exists(GEMINI_SKILLS_DIR):
        print(f"Creating {GEMINI_SKILLS_DIR}...")
        os.makedirs(GEMINI_SKILLS_DIR, exist_ok=True)

    # Find all SKILL.md files in sovereign skills dir
    skill_mds = glob.glob(os.path.join(SOVEREIGN_SKILLS_DIR, "**/SKILL.md"), recursive=True)
    
    found_skills = {}
    for filepath in skill_mds:
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()
        
        fm = parse_frontmatter(content)
        skill_dir = os.path.dirname(filepath)
        # Use name from frontmatter if available, otherwise directory name
        skill_name = fm.get('name', os.path.basename(skill_dir))
        
        # If it's _shared or something we don't want to link as a skill
        if skill_name == "_shared":
            continue
            
        if skill_name not in found_skills:
            found_skills[skill_name] = skill_dir
        else:
            # If duplicate, prefer the one with a shorter path (more likely to be the "canonical" one)
            if len(skill_dir) < len(found_skills[skill_name]):
                found_skills[skill_name] = skill_dir

    print(f"Found {len(found_skills)} skills in {SOVEREIGN_SKILLS_DIR}")

    # Remove broken or old vault links in ~/.gemini/skills
    if os.path.exists(GEMINI_SKILLS_DIR):
        for filename in os.listdir(GEMINI_SKILLS_DIR):
            link_path = os.path.join(GEMINI_SKILLS_DIR, filename)
            if os.path.islink(link_path):
                target = os.readlink(link_path)
                if not os.path.exists(target) or "hakaishin-vault" in target:
                    print(f"Removing invalid link: {filename} -> {target}")
                    os.remove(link_path)

    # Create new links
    for name, path in found_skills.items():
        # Sanitize name for filesystem
        safe_name = name.replace(" ", "-").lower()
        link_path = os.path.join(GEMINI_SKILLS_DIR, safe_name)
        
        if os.path.exists(link_path) or os.path.islink(link_path):
            if os.path.islink(link_path):
                if os.readlink(link_path) == path:
                    continue
                os.remove(link_path)
            else:
                print(f"Skipping {safe_name}: path already exists and is not a link.")
                continue
        
        print(f"Linking {safe_name} -> {path}")
        os.symlink(path, link_path)

    # Special case for _shared which is often needed
    shared_path = os.path.join(SOVEREIGN_SKILLS_DIR, "_shared")
    shared_link = os.path.join(GEMINI_SKILLS_DIR, "_shared")
    if os.path.exists(shared_path) and not os.path.exists(shared_link):
        print(f"Linking _shared -> {shared_path}")
        os.symlink(shared_path, shared_link)

if __name__ == "__main__":
    fix_symlinks()
