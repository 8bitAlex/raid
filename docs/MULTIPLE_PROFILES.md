# Multiple Profiles Support

The raid profile system now supports multiple profiles in a single file, making it easier to manage related profiles together.

## Supported Formats

### YAML Documents
Multiple profiles can be defined in a single YAML file using document separators (`---`):

```yaml
name: development
repositories:
  - name: frontend
    path: ~/Developer/frontend
    url: https://github.com/company/frontend.git
  - name: backend
    path: ~/Developer/backend
    url: https://github.com/company/backend.git
---
name: personal
repositories:
  - name: blog
    path: ~/Developer/blog
    url: https://github.com/username/blog.git
  - name: dotfiles
    path: ~/Developer/dotfiles
    url: https://github.com/username/dotfiles.git
---
name: open-source
repositories:
  - name: raid
    path: ~/Developer/raid
    url: https://github.com/8bitAlex/raid.git
```

### JSON Arrays
Multiple profiles can also be defined in a single JSON file using arrays:

```json
[
  {
    "name": "development",
    "repositories": [
      {
        "name": "frontend",
        "path": "~/Developer/frontend",
        "url": "https://github.com/company/frontend.git"
      },
      {
        "name": "backend",
        "path": "~/Developer/backend",
        "url": "https://github.com/company/backend.git"
      }
    ]
  },
  {
    "name": "personal",
    "repositories": [
      {
        "name": "blog",
        "path": "~/Developer/blog",
        "url": "https://github.com/username/blog.git"
      }
    ]
  }
]
```

## Usage

### Adding Multiple Profiles
To add multiple profiles from a single file:

```bash
raid profile add profiles.yaml
```

This will:
1. Extract all profiles from the file
2. Validate each profile against the schema
3. Add only the profiles that don't already exist
4. Report any existing profiles that were skipped
5. Set the first new profile as active if no active profile exists

### Example Output

When adding multiple profiles:

```bash
$ raid profile add multiple-profiles.yaml
Profiles development, personal, open-source have been successfully added from multiple-profiles.yaml. Profile 'development' has been set as active
```

When some profiles already exist:

```bash
$ raid profile add multiple-profiles.yaml
Profiles already exist: development
Profiles personal, open-source have been successfully added from multiple-profiles.yaml
```

## Features

### Auto-Activation
- If no active profile exists, the first profile in the file is automatically set as active
- If an active profile already exists, no auto-activation occurs

### Duplicate Handling
- Existing profiles are detected and reported
- Only new profiles are added
- The operation continues even if some profiles already exist

### Validation
- Each profile is individually validated against the JSON schema
- Invalid profiles are reported with specific error messages
- Empty YAML documents are automatically skipped

### Backward Compatibility
- Single profile files continue to work as before
- The `ExtractProfileName` function still works for single profiles
- Existing functionality is preserved

## Example Files

See the example files in the `docs/examples/` directory:
- `multiple-profiles.yaml` - YAML example with document separators
- `multiple-profiles.json` - JSON example with arrays
- `example.raid.yaml` - Single profile example

## Error Handling

The system provides clear error messages for various scenarios:

- **Missing name field**: "profile document X is missing required 'name' field"
- **Invalid format**: "profile document X 'name' field must be a string"
- **Empty name**: "profile document X 'name' field cannot be empty"
- **Invalid YAML**: "invalid YAML document X"
- **Invalid JSON**: "invalid JSON format"
- **Existing profiles**: "Profiles already exist: profile1, profile2"

## Implementation Details

### YAML Processing
- Files are split by `---` document separators
- Empty documents are automatically skipped
- Each document is parsed and validated individually

### JSON Processing
- Supports both single objects and arrays
- Automatically detects the format
- Handles nested profile structures

### Validation
- Each profile is validated against the JSON schema
- Schema validation occurs after parsing
- Detailed error messages include profile/document numbers
