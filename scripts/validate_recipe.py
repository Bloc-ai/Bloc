#!/usr/bin/env python3
import sys
import os
import re

try:
    import yaml
except ImportError:
    print("PyYAML is not installed. Run 'pip install PyYAML'")
    sys.exit(1)

def validate_recipe(file_path):
    if not os.path.exists(file_path):
        print(f"Error: File '{file_path}' does not exist.")
        return False

    if not file_path.endswith('.yaml') and not file_path.endswith('.yml'):
        print(f"Error: File '{file_path}' must have a .yaml or .yml extension.")
        return False

    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            data = yaml.safe_load(f)
    except yaml.YAMLError as exc:
        print(f"Error: Failed to parse YAML file '{file_path}':\n{exc}")
        return False
    except Exception as exc:
        print(f"Error: Failed to read file '{file_path}':\n{exc}")
        return False

    if not data:
        print(f"Error: File '{file_path}' is empty.")
        return False

    # 1. Validate Schema version
    if 'schema' not in data:
        print("Error: Missing 'schema' field.")
        return False
    if data['schema'] != 'bloc/v1':
        print(f"Error: Unsupported schema version '{data['schema']}'. Expected 'bloc/v1'.")
        return False

    # 2. Validate Layer 1 metadata structures
    required_sections = ['metadata', 'model', 'engine', 'hardware']
    for sec in required_sections:
        if sec not in data or not isinstance(data[sec], dict):
            print(f"Error: Missing or invalid section '{sec}'.")
            return False

    # Metadata Validation
    metadata = data['metadata']
    if 'name' not in metadata or not metadata['name']:
        print("Error: Missing 'metadata.name'.")
        return False
    
    name_pattern = re.compile(r'^[a-z0-9\-]+$')
    if not name_pattern.match(str(metadata['name'])):
        print(f"Error: Invalid 'metadata.name' value '{metadata['name']}'. Must be lowercase alphanumeric and hyphens only.")
        return False

    if 'description' not in metadata or not metadata['description']:
        print("Error: Missing 'metadata.description'.")
        return False

    # Model Validation
    model = data['model']
    required_model_fields = ['source', 'file', 'download_url']
    for field in required_model_fields:
        if field not in model or not model[field]:
            print(f"Error: Missing 'model.{field}'.")
            return False

    download_url = str(model['download_url'])
    if not download_url.startswith('https://huggingface.co/'):
        print(f"Error: 'model.download_url' must be a valid Hugging Face URL starting with 'https://huggingface.co/'. Got: {download_url}")
        return False

    # Engine Validation
    engine = data['engine']
    if 'name' not in engine or engine['name'] != 'llama.cpp':
        print(f"Error: 'engine.name' must be 'llama.cpp'.")
        return False

    # Hardware Validation
    hardware = data['hardware']
    required_hardware_fields = ['min_vram', 'target_platform']
    for field in required_hardware_fields:
        if field not in hardware or not hardware[field]:
            print(f"Error: Missing 'hardware.{field}'.")
            return False

    valid_vram = ['4GB', '8GB', '12GB', '16GB', '24GB', 'Unified']
    if hardware['min_vram'] not in valid_vram:
        print(f"Error: Invalid 'hardware.min_vram' value '{hardware['min_vram']}'. Expected one of: {', '.join(valid_vram)}")
        return False

    valid_platforms = ['cuda', 'metal', 'rocm', 'cpu', 'vulkan']
    if hardware['target_platform'] not in valid_platforms:
        print(f"Error: Invalid 'hardware.target_platform' value '{hardware['target_platform']}'. Expected one of: {', '.join(valid_platforms)}")
        return False

    # 3. Validate Layer 2 layout if present
    if 'engine_config' in data:
        config = data['engine_config']
        if not isinstance(config, dict):
            print("Error: 'engine_config' section must be a key-value mapping.")
            return False

    print(f"✅ Success: '{file_path}' is a valid Bloc recipe!")
    return True

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Usage: validate_recipe.py <path_to_recipe_yaml>")
        sys.exit(1)

    recipe_path = sys.argv[1]
    success = validate_recipe(recipe_path)
    if not success:
        sys.exit(1)
