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

def remove_module(arr, mod):
    if not isinstance(arr, list):
        return arr
    return [x for x in arr if x != mod]

def find_drawer_group_key(cfg_obj):
    # Prefer Omarchy's tray expander if present.
    if isinstance(cfg_obj.get("group/tray-expander"), dict) and isinstance(cfg_obj["group/tray-expander"].get("modules"), list):
        return "group/tray-expander"
    # Otherwise, look for any group/* with a modules list (drawer-like).
    for k, v in cfg_obj.items():
        if not isinstance(k, str) or not k.startswith("group/"):
            continue
        if isinstance(v, dict) and isinstance(v.get("modules"), list):
            return k
    return None

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
    else:
        # Create it if missing
        cfg["custom/wizado"] = {
            "format": "{}",
            "return-type": "json",
            "exec": str(pathlib.Path.home() / ".config/waybar/scripts/wizado-status.sh"),
            "interval": 2,
            "on-click": term_cmd + " " + str(pathlib.Path.home() / ".local/share/steam-launcher/wizado-menu"),
            "tooltip": True
        }

    # Prefer placing inside a collapsed tray drawer group if present.
    group_key = find_drawer_group_key(cfg)
    if group_key:
        group = cfg.get(group_key, {})
        mods = group.get("modules", [])
        if isinstance(mods, list):
            if "custom/wizado" not in mods:
                if "tray" in mods:
                    i = mods.index("tray") + 1
                    mods = mods[:i] + ["custom/wizado"] + mods[i:]
                else:
                    mods = mods + ["custom/wizado"]
                    if mods and mods[0] == "custom/wizado":
                        mods = ["custom/expand-icon"] + mods
            group["modules"] = mods
            cfg[group_key] = group
        for list_key in ("modules-right", "modules-left", "modules-center"):
            cfg[list_key] = remove_module(cfg.get(list_key, []), "custom/wizado")
        
    out = json.dumps(cfg, indent=2)
    path.write_text(out + "\n", encoding="utf-8")
    print("Patch success")
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)

