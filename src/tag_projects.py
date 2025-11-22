#!/usr/bin/env python3
"""
Script to tag projects containing exported mixdowns with TAG_NAME tag.
"""

import os
import sys
import xattr
import plistlib

# Configuration
TAG_NAME = "Blue"  # Change this to use a different tag


def set_tags(filepath, tags):
    """
    Set Finder tags on a file or directory.
    
    Args:
        filepath: Path to the file or directory
        tags: List of tag names to set
    """
    try:
        # Convert tags to binary plist format
        tags_data = plistlib.dumps(tags, fmt=plistlib.FMT_BINARY)
        # Set the extended attribute that stores Finder tags
        xattr.setxattr(filepath, 'com.apple.metadata:_kMDItemUserTags', tags_data)
        print(f"✓ Tagged: {filepath}")
        return True
    except Exception as e:
        print(f"✗ Error tagging {filepath}: {e}")
        return False


def get_tags(filepath):
    """
    Get existing Finder tags from a file or directory.
    
    Args:
        filepath: Path to the file or directory
    
    Returns:
        List of existing tags, or empty list if none
    """
    try:
        tags_data = xattr.getxattr(filepath, 'com.apple.metadata:_kMDItemUserTags')
        tags = plistlib.loads(tags_data)
        return tags
    except (OSError, KeyError):
        # No tags exist yet
        return []


def add_tag(filepath, new_tag):
    """
    Add a tag to existing tags without removing others.
    
    Args:
        filepath: Path to the file or directory
        new_tag: Tag name to add
    """
    existing_tags = get_tags(filepath)
    
    # Add new tag if it doesn't already exist
    if new_tag not in existing_tags:
        existing_tags.append(new_tag)
        return set_tags(filepath, existing_tags)
    else:
        print(f"○ Already tagged: {filepath}")
        return True


def remove_tag(filepath, tag_to_remove):
    """
    Remove a specific tag from a file or directory.
    
    Args:
        filepath: Path to the file or directory
        tag_to_remove: Tag name to remove
    """
    existing_tags = get_tags(filepath)
    
    # Remove the tag if it exists
    if tag_to_remove in existing_tags:
        existing_tags.remove(tag_to_remove)
        if existing_tags:
            # Set remaining tags
            set_tags(filepath, existing_tags)
        else:
            # Remove the attribute entirely if no tags left
            try:
                xattr.removexattr(filepath, 'com.apple.metadata:_kMDItemUserTags')
            except OSError:
                pass
        print(f"✓ Removed tag from: {filepath}")
        return True
    else:
        print(f"○ Tag not present: {filepath}")
        return False


def process_directory(base_path, remove_mode=False):
    """
    Process all directories under the given path.
    Tag (or untag) any directory that contains a "Mixdown" or "Exported Files" subdirectory.
    
    Args:
        base_path: The root directory path to process
        remove_mode: If True, remove tags instead of adding them
    """
    if not os.path.isdir(base_path):
        print(f"Error: {base_path} is not a valid directory")
        return
    
    action = "Removing tags from" if remove_mode else "Scanning"
    print(f"{action}: {base_path}\n")
    processed_count = 0
    
    # Subdirectories to look for
    target_subdirs = ["Mixdown", "Exported Files"]
    
    # Iterate through all items in the base path
    try:
        for item in os.listdir(base_path):
            item_path = os.path.join(base_path, item)
            
            # Only process directories
            if os.path.isdir(item_path):
                # Check if any of the target subdirectories exist
                found_match = False
                for subdir_name in target_subdirs:
                    subdir_path = os.path.join(item_path, subdir_name)
                    if os.path.isdir(subdir_path):
                        found_match = True
                        break
                
                # Tag or untag the parent directory if match found
                if found_match:
                    if remove_mode:
                        if remove_tag(item_path, TAG_NAME):
                            processed_count += 1
                    else:
                        if add_tag(item_path, TAG_NAME):
                            processed_count += 1
    
    except PermissionError as e:
        print(f"Error: Permission denied accessing {base_path}")
        return
    
    print(f"\n{'='*50}")
    action_word = "Untagged" if remove_mode else "Tagged"
    print(f"Complete! {action_word} {processed_count} director{'y' if processed_count == 1 else 'ies'}.")


def main():
    """Main entry point for the script."""
    if len(sys.argv) < 2 or len(sys.argv) > 3:
        print("Usage: python tag_directories.py [-r] <directory_path>")
        print("\nOptions:")
        print("  -r    Remove tags instead of adding them")
        print("\nExamples:")
        print("  python tag_directories.py /Users/username/Projects")
        print("  python tag_directories.py -r /Users/username/Projects")
        sys.exit(1)
    
    # Check for -r flag
    remove_mode = False
    if sys.argv[1] == "-r":
        remove_mode = True
        directory_path = sys.argv[2] if len(sys.argv) == 3 else None
        if not directory_path:
            print("Error: Directory path required after -r flag")
            sys.exit(1)
    else:
        directory_path = sys.argv[1]
    
    process_directory(directory_path, remove_mode)


if __name__ == "__main__":
    main()