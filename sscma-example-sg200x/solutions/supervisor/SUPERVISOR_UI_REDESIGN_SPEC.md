# UI Specification Document
## Scope
This document defines **UI-only changes**.  
There are **no backend logic changes**.  
All existing APIs, data models, and behaviors remain unchanged.  
This spec is intended for an AI or engineer to derive a **task list**.

Order of items below does **not** indicate priority or sequence.

---

## 1. Hostname Display and Editing

### 1.1 Top Bar Hostname Visibility
- Keep the **existing edit icon** in its current location.
- Improve the **visibility and clarity** of the edit icon so it is easier to notice.
- Update the top bar text to display:
  - **Camera Name: <hostname>**
- The hostname must still be editable from the UI.
- Do **not** display the IP address on the top bar.

### 1.2 Network Menu Hostname Section
- Add or retain a **Hostname configuration section** inside the **Network menu**.
- This section allows changing the hostname.

---

## 2. Upload Files Window Readability

### 2.1 General Readability
- Improve readability of the upload files window.
- Increase visibility and clarity of icons and labels.
- Ensure icons and text are clearly distinguishable at a glance.

### 2.2 Upload Action Window
- On the **second window** where the **actual Upload button** is displayed:
  - Make the Upload button significantly more readable.
  - Improve contrast, size, and visual emphasis so the primary action is obvious.

---

## 3. Recording Menu Section

### 3.1 Menu Entry
- Add a new **Recording** section to the main menu.

### 3.2 Recording Location
- Provide a **recording location selector** with the following options:
  - SD Card
  - Local Storage

### 3.3 Recording Mode Selection
Provide the following selectable modes:
- Motion based recording
- Constant recording
- Recording during selected times only

### 3.4 Time Based Recording Configuration
Only shown when **Recording during selected times only** is selected.

#### Requirements:
- Day of week checkboxes
  - Sunday through Saturday
- Start time selector
- End time selector
- If start time and end time are both **00:00**, treat as **24 hour recording** for that day.

---

## 4. LED Configuration Page

### 4.1 Menu Entry
- Add a **LED Configuration** page to the menu.

### 4.2 LED Controls
Provide individual toggle controls for:
- White Light LEDs
- Blue LED
- Red LED
- Green LED

Each LED must have an independent on or off toggle.

---

## 5. Updates Menu Section

### 5.1 Updates Menu Entry
- Add an **Updates** section to the menu.

### 5.2 Camera OS Updates Card
- Combine **Update** and **Beta channel** into a single card.
- Rename the card to:
  - **Camera OS Updates**

#### Channel Selection
- Replace the bottom scroller with a clearer and more discoverable selection control.

### 5.3 Camera Model Updates Card
- Add a second card for **Camera Model Updates**.
- This card must have the **same features and layout** as Camera OS Updates.

### 5.4 Update Check Frequency
Add a selection for update check frequency:
- Every 30 minutes
- Daily
- Weekly
  - Weekly requires a **day of week selection**
- Manual only

---

## 6. About Menu Section

### 6.1 Menu Entry
- Add an **About** section to the menu.

### 6.2 Links
Provide links to:
- Open Source Licenses
- Data Collection Policy

### 6.3 Footer Message
- Add a short friendly message such as:
  - Made with thought for you, the user.
  - We care more about your security than the original developer.
- Tone should be friendly and reassuring.

---

## 7. Storage Usage Display

### 7.1 Storage Information Section
- Add a section that shows:
  - Total storage space
  - Used storage space

### 7.2 Storage Source Awareness
- Storage information must update based on selection:
  - Local files
  - SD card

---

## 8. WiFi Network List Behavior

### 8.1 Network Menu Adjustment
- In the Network menu, limit visible WiFi networks to **6 entries**.
- If more than 6 networks exist:
  - Show a scroll bar
  - Allow scrolling to view the remaining networks
- Prevent the list from expanding the window height excessively.

---

## Notes
- No backend logic changes are allowed.
- UI behavior must map to existing backend functionality.
- This document is designed to be machine readable and task oriented.
