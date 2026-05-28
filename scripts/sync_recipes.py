#!/usr/bin/env python3
import sys
import os
import json
import urllib.request
import urllib.parse

try:
    import yaml
except ImportError:
    print("PyYAML is not installed. Run 'pip install PyYAML'")
    sys.exit(1)

# Retrieve Supabase Credentials from environment variables
SUPABASE_URL = os.environ.get("SUPABASE_URL")
SUPABASE_SERVICE_KEY = os.environ.get("SUPABASE_SERVICE_KEY")

if not SUPABASE_URL or not SUPABASE_SERVICE_KEY:
    print("Error: SUPABASE_URL and SUPABASE_SERVICE_KEY environment variables must be set.")
    sys.exit(1)

# Normalize Supabase base URL
SUPABASE_URL = SUPABASE_URL.rstrip('/')

def make_supabase_request(path, method="GET", headers=None, body=None):
    url = f"{SUPABASE_URL}{path}"
    
    default_headers = {
        "apikey": SUPABASE_SERVICE_KEY,
        "Authorization": f"Bearer {SUPABASE_SERVICE_KEY}",
        "Content-Type": "application/json"
    }
    if headers:
        default_headers.update(headers)
        
    data = None
    if body:
        data = json.dumps(body).encode('utf-8')
        
    req = urllib.request.Request(url, headers=default_headers, data=data, method=method)
    
    try:
        with urllib.request.urlopen(req) as resp:
            status = resp.status
            response_content = resp.read().decode('utf-8')
            if response_content:
                return status, json.loads(response_content)
            return status, None
    except urllib.error.HTTPError as e:
        err_body = e.read().decode('utf-8')
        print(f"HTTP Error {e.code} on {method} {path}: {err_body}")
        return e.code, None
    except Exception as e:
        print(f"Connection Error on {method} {path}: {e}")
        return 500, None

def get_user_auth_id(username):
    # Fetch profile associated with this username to get their auth_id
    encoded_username = urllib.parse.quote(username)
    path = f"/rest/v1/profiles?username=eq.{encoded_username}&select=auth_id"
    
    status, data = make_supabase_request(path, method="GET")
    if status == 200 and data and len(data) > 0:
        return data[0].get("auth_id")
    return None

def sync_recipe(file_path):
    print(f"\nProcessing file: {file_path}")
    
    # 1. Extract creator username from directory structure: recipes/[username]/[recipe-name].yaml
    parts = file_path.split(os.sep)
    # If the path starts with relative directories (e.g. ./recipes/arnav/...)
    if parts[0] == '.':
        parts = parts[1:]
        
    if len(parts) < 3 or parts[0] != 'recipes':
        print(f"Skipping {file_path}: File path must match 'recipes/[username]/[recipe-name].yaml'")
        return False
        
    creator_username = parts[1]
    
    # 2. Parse YAML content
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            raw_content = f.read()
            data = yaml.safe_load(raw_content)
    except Exception as e:
        print(f"Failed to parse {file_path}: {e}")
        return False

    # 3. Retrieve creator's auth_id from profiles table
    auth_id = get_user_auth_id(creator_username)
    if not auth_id:
        print(f"Skipping {file_path}: Creator username '{creator_username}' does not have a profile registered on Bloc Hub.")
        print("Please ensure you have signed in and set up your profile first.")
        return False

    # 4. Prepare recipe payload mapping to Database Schema
    metadata = data.get('metadata', {})
    model = data.get('model', {})
    hardware = data.get('hardware', {})
    engine = data.get('engine', {})

    recipe_payload = {
        "auth_id": auth_id,
        "creator": creator_username,
        "name": metadata.get('name'),
        "description": metadata.get('description'),
        "base_model": model.get('source'),
        "min_vram": hardware.get('min_vram'),
        "target_platform": hardware.get('target_platform'),
        "yaml_content": raw_content,
        "tested_commit": engine.get('tested_commit'),
        "compat_builds": [engine.get('tested_commit')] if engine.get('tested_commit') else []
    }

    # 5. POST/Upsert to Supabase
    # PostgREST uses 'Prefer: resolution=merge-duplicates' to execute an upsert on unique constraint
    headers = {
        "Prefer": "resolution=merge-duplicates"
    }
    
    status, _ = make_supabase_request("/rest/v1/recipes", method="POST", headers=headers, body=recipe_payload)
    if status in [200, 201]:
        print(f"✅ Successfully synced recipe '{creator_username}/{metadata.get('name')}' to database!")
        return True
    else:
        print(f"❌ Failed to sync recipe '{creator_username}/{metadata.get('name')}' to database. Status code: {status}")
        return False

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Usage: sync_recipes.py <path_to_recipe_yaml> [path_to_recipe_yaml_2 ...]")
        sys.exit(0)

    files_to_sync = sys.argv[1:]
    success_count = 0
    
    for f in files_to_sync:
        if sync_recipe(f):
            success_count += 1
            
    print(f"\nSummary: Synced {success_count}/{len(files_to_sync)} recipes.")
    if success_count < len(files_to_sync):
        sys.exit(1)
