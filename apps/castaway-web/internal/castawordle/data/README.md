# Castawordle allowed guesses

Source: [English Speller Database (SCOWL)](https://github.com/en-wl/wordlist), pinned revision `1e5b7d3a72f47a71da5d28686c1dd4b397178485`.

Configuration: size 70; American and British spellings (`A,B,Z`), variant level 1; no abbreviations or special categories. Keep only lowercase ASCII words of 4–8 letters, uppercase and deduplicate. This excludes capitalized names, punctuation and accented entries without inventing spellings. Inflections and ordinary US/UK variants remain valid. There are **59,212** entries, including `SWADDLE`, `SWADDLED`, `SWADDLES`, `COLOR` and `COLOUR`. No manual word additions. This is not the NYT Wordle list or a strict superset of the previous CMUdict snapshot.

Reproduce from the pinned upstream checkout with Python 3 and SQLite already available:

```sh
make
./scowl word-list 70 A,B,Z 1 --wo-poses=abbr --categories= > /tmp/scowl-70.txt
python3 - <<'PY'
from pathlib import Path
import re
words = sorted({word.upper() for word in Path('/tmp/scowl-70.txt').read_text().splitlines()
                if re.fullmatch('[a-z]{4,8}', word)})
Path('/tmp/words.txt').write_text('\n'.join(words) + '\n')
PY
```

Copy `/tmp/words.txt` to this directory. `LICENSE` retains upstream's copyright/permission text before the first `===` separator, as prescribed for generated, non-Australian lists at size 70. No runtime downloads or dictionary service. Operators choose familiar answers separately from the broader allowed-guess list.
