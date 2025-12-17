import json, re, pathlib, sys, shutil

path = pathlib.Path.home() / ".config/waybar/config.jsonc"

def get_terminal_cmd():
    terms = ["ghostty", "alacritty", "kitty", "foot", "gnome-terminal", "konsole"]
    for t in terms:
        if shutil.which(t):
            if t == "gnome-terminal": return t + " --"
            if t == "konsole": return t + " -e"
            return t + " -e"
    return "alacritty -e"

try:
    print(f"Reading {path}")
    raw = path.read_text(encoding="utf-8")
    raw2 = re.sub(r"//.*?$", "", raw, flags=re.M)
    cfg = json.loads(raw2)

    term_cmd = get_terminal_cmd()
    
    # Update the module config
    if "custom/wizado" in cfg:
        print("Updating custom/wizado module...")
        cfg["custom/wizado"]["format"] = "{}"  # Fix the icon format issue
        # Ensure command is correct too
        cfg["custom/wizado"]["on-click"] = term_cmd + " " + str(pathlib.Path.home() / ".local/share/steam-launcher/wizado-menu")
        
    out = json.dumps(cfg, indent=2)
    path.write_text(out + "\n", encoding="utf-8")
    print("Patch success")
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)

