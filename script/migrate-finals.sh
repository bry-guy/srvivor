#!/bin/bash
# Migrate finals to new structure

for season in {44..49}; do
    old_path="finals/${season}.txt"
    new_path="drafts/${season}/final.txt"

    if [ -f "$old_path" ] && [ ! -f "$new_path" ]; then
        mkdir -p "drafts/${season}"
        cp "$old_path" "$new_path"
        echo "Migrated finals/${season}.txt to drafts/${season}/final.txt"
    fi
done

echo "Migration complete. Old finals/ directory preserved."