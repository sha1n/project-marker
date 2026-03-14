# Project Marker

This project contains a script that labels directories based on their content, specifically designed for identifying projects with exported mixdowns on macOS.

## Features

- **Automated Tagging:** Automatically identifies directories containing "Mixdown" or "Exported Files" subdirectories.
- **macOS Finder Tags:** Uses native macOS Finder tags (extended attributes) for visual identification.
- **Multiple Modes:**
  - **Tagging:** Add a tag (default: "Blue") to matching directories.
  - **Removal:** Remove a specific tag from matching directories using the `-r` flag.
- **Customizable:** The tag name can be easily modified in the script's `TAG_NAME` variable.

## Execution Instructions

### Prerequisites

- macOS (as it uses Finder tags/extended attributes)
- Python 3.13 or higher
- [Poetry](https://python-poetry.org/docs/#installation) for dependency management

### Setup

1. Clone this repository.
2. Install dependencies:
   ```bash
   poetry install
   ```

### Running the Script

You can run the script using `poetry run`:

#### Tagging matching directories:
```bash
poetry run src/tag_projects.py /path/to/your/projects
```

#### Removing tags from matching directories:
```bash
poetry run src/tag_projects.py -r /path/to/your/projects
```

## Documentation

### How it works
The script scans all immediate subdirectories of the provided `directory_path`. It looks for sub-folders named either `Mixdown` or `Exported Files`. If such a folder is found, the script adds a Finder tag to the parent directory.

This is particularly useful for music production or video editing workflows where you want to visually mark projects that have been "exported" or "mixed down".

### Configuration
You can change the tag name by editing `src/tag_projects.py`:
```python
TAG_NAME = "Blue"  # Change this to use a different tag
```
Common tag names on macOS include "Red", "Orange", "Yellow", "Green", "Blue", "Purple", and "Gray".
