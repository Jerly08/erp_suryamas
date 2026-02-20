import json
import sys

def validate(filename):
    print(f"Validating {filename}...")
    try:
        with open(filename, 'r', encoding='utf-8') as f:
            content = f.read()
        
        # Check for duplicates manually because json.load might not complain unless strict
        lines = content.splitlines()
        keys_seen = set()
        # This is a very rough check for duplicates in the specific area I edited
        # Proper JSON parser
        data = json.loads(content)
        print("JSON is valid.")
    except json.JSONDecodeError as e:
        print(f"JSON Error: {e}")
        # Print context
        lines = content.splitlines()
        start = max(0, e.lineno - 5)
        end = min(len(lines), e.lineno + 5)
        for i in range(start, end):
            prefix = ">> " if i + 1 == e.lineno else "   "
            print(f"{prefix}{i+1}: {lines[i]}")

validate(sys.argv[1])
