# Let's first load the torrent file and extract its raw bencoded data. 
# Then we'll try to isolate the 'info' dictionary and hash it.

import hashlib

# Read the uploaded torrent file
file_path = '/mnt/data/leaves.torrent'
with open(file_path, 'rb') as f:
    torrent_data = f.read()

# Helper function to extract the "info" dictionary from the bencoded torrent data
def extract_info_section(bencoded_data):
    info_key = b'4:info'
    start_index = bencoded_data.find(info_key)
    if start_index == -1:
        raise ValueError("'info' key not found in the bencoded data")
    
    # Move past the "4:info" key to the dictionary itself
    start_index += len(info_key)

    # Now we need to parse the dictionary and find where it ends.
    # The simplest approach would be to manually parse the structure here.
    # We'll do this in a simplified way by counting nested dictionaries, lists, etc.
    depth = 0
    i = start_index
    while i < len(bencoded_data):
        if bencoded_data[i:i+1] == b'd' or bencoded_data[i:i+1] == b'l':
            depth += 1
        elif bencoded_data[i:i+1] == b'e':
            depth -= 1
            if depth == 0:
                return bencoded_data[start_index:i+1]
        i += 1
    raise ValueError("Could not determine the end of the 'info' dictionary")

# Extract the info section
info_section = extract_info_section(torrent_data)

# Compute the SHA-1 hash of the extracted info section
info_hash = hashlib.sha1(info_section).hexdigest()
info_hash
