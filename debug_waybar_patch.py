import json, re, pathlib, sys, shutil

path = pathlib.Path.home() / ".config/waybar/config.jsonc"

def get_terminal_cmd():
    # Helper to find a terminal command (basic version of bash logic)
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
    # Very small jsonc stripper (removes // comments). Omarchy config.jsonc is plain JSON today.
    raw2 = re.sub(r"//.*?$", "", raw, flags=re.M)
    cfg = json.loads(raw2)

    def remove_module(arr, mod):
        if not isinstance(arr, list):
            return arr
        return [x for x in arr if x != mod]

    def inject_right(arr):
        if not isinstance(arr, list):
            return ["custom/wizado"]
        if "custom/wizado" in arr:
            return arr
        for target in ["bluetooth", "network", "pulseaudio", "cpu", "battery"]:
            if target in arr:
                i = arr.index(target)
                return arr[:i] + ["custom/wizado"] + arr[i:]
        return arr + ["custom/wizado"]

    def find_drawer_group_key(cfg_obj):
        if isinstance(cfg_obj.get("group/tray-expander"), dict) and isinstance(cfg_obj["group/tray-expander"].get("modules"), list):
            return "group/tray-expander"
        for k, v in cfg_obj.items():
            if not isinstance(k, str) or not k.startswith("group/"):
                continue
            if isinstance(v, dict) and isinstance(v.get("modules"), list):
                return k
        return None

    print("Injecting module...")
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
    else:
        cfg["modules-right"] = inject_right(cfg.get("modules-right", []))

    term_cmd = get_terminal_cmd()
    print(f"Detected terminal: {term_cmd}")

    cfg["custom/wizado"] = {
        "format": "{}",
        "return-type": "json",
        "exec": str(pathlib.Path.home() / ".config/waybar/scripts/wizado-status.sh"),
        "interval": 2,
        "on-click": term_cmd + " " + str(pathlib.Path.home() / ".local/share/steam-launcher/wizado-menu"),
        "on-click-right": str(pathlib.Path.home() / ".local/share/steam-launcher/enter-gamesmode") + " --mode tty",
        "on-click-middle": str(pathlib.Path.home() / ".local/share/steam-launcher/leave-gamesmode"),
        "tooltip": True
    }

    out = json.dumps(cfg, indent=2)
    print("Patching config file...")
    path.write_text(out + "\n", encoding="utf-8")
    print("Patch success")
except Exception as e:
    print(f"Skipping Waybar patch due to error: {e}", file=sys.stderr)
    import traceback
    traceback.print_exc()

