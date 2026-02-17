# Programming Specification: Supervisor UI Redesign

## Document Overview

**Source Document:** `SUPERVISOR_UI_REDESIGN_SPEC.md`  
**Scope:** Frontend UI changes only - no backend modifications  
**Tech Stack:** React + TypeScript + Ant Design + Tailwind CSS  
**Target Directory:** `www/src/`

---

## Summary of Understanding

This specification translates the UI redesign requirements into concrete programming tasks. The project is a camera supervisor web interface built with React/TypeScript. All changes are UI-only; existing APIs and data models remain unchanged.

---

## Feature Breakdown

### Feature 1: Hostname Display and Editing

#### 1.1 Top Bar Modifications
**Location:** `www/src/layout/mobile/header/index.tsx` (and PC equivalent)

**Requirements:**
- Modify top bar text format from current display to: `Camera Name: <hostname>`
- Remove IP address display from top bar
- Enhance edit icon visibility (increase size, add hover effects, improve contrast)
- Maintain existing edit functionality

**Implementation Notes:**
- The edit icon should remain in its current position
- Consider adding a tooltip on hover: "Edit Camera Name"
- Use existing hostname API endpoint for data

#### 1.2 Network Menu Hostname Section
**Location:** `www/src/views/network/index.tsx`

**Requirements:**
- Add/retain a "Hostname" configuration card within the Network view
- Provide text input field for hostname editing
- Include Save/Apply button
- Show validation feedback for invalid hostnames

---

### Feature 2: Upload Files Window Improvements

#### 2.1 Upload Modal Dialog Readability
**Location:** `www/src/views/files/index.tsx` (and related modal components)

**Current Problem (from screenshot):**
The "Upload Files" modal dialog has severe readability issues:
- Modal background is semi-transparent/dark, making content hard to read
- Selected file names (e.g., "watch_20251225-124133") are barely visible (gray text on dark background)
- Poor contrast throughout the modal
- The paperclip icon and file list are difficult to see

**Requirements:**
- **Modal Background:** Change to solid, opaque background (white or light gray) instead of semi-transparent dark
- **File List Text:** Use high-contrast text color (dark text on light background)
- **Selected Files Display:**
  - Increase font size for file names
  - Use clear, readable text color (e.g., `#333` or `#000` on light background)
  - Add clear visual separation between listed files
- **Icons:** Ensure paperclip/attachment icons have sufficient contrast
- **"Select files" Button:** Maintain current visibility (appears readable in screenshot)

#### 2.2 Upload Action Button Enhancement
**Location:** Upload modal/dialog component

**Current State (from screenshot):**
The "Upload" button (blue, right side) and "Cancel" button are visible but could be more prominent.

**Requirements:**
- Make the primary "Upload" button more prominent:
  - Increase button size slightly
  - Ensure high-contrast colors are maintained
  - Consider adding visual emphasis (shadow or border)
- Ensure clear visual distinction between "Cancel" (secondary) and "Upload" (primary) buttons
- "Upload" button should be immediately identifiable as the primary action

#### 2.3 Modal Styling Specifications
```css
/* Recommended modal styling */
.upload-modal {
  background-color: #ffffff;  /* Solid white background */
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
}

.upload-modal .file-list-item {
  color: #333333;  /* Dark text for readability */
  font-size: 14px;
  padding: 8px 12px;
  border-bottom: 1px solid #e0e0e0;
}

.upload-modal .file-icon {
  color: #666666;  /* Visible icon color */
}

.upload-modal .upload-button {
  background-color: #4f46e5;  /* Primary color */
  color: #ffffff;
  font-weight: 600;
  padding: 10px 24px;
}
```

---

### Feature 3: Recording Menu Section (NEW)

#### 3.1 Menu Entry
**Location:** `www/src/layout/mobile/sidebar/index.tsx` (and PC equivalent)

**Requirements:**
- Add new "Recording" menu item to sidebar navigation
- Create appropriate icon for Recording section
- Add route configuration for `/recording` path

#### 3.2 Recording View Component
**Location:** `www/src/views/recording/index.tsx` (NEW FILE)

**Requirements:**
Create a new view with the following sections:

**A. Recording Location Selector**
- Radio button group or dropdown with options:
  - "SD Card"
  - "Local Storage"

**B. Recording Mode Selection**
- Radio button group with options:
  - "Motion based recording"
  - "Constant recording"
  - "Recording during selected times only"

**C. Time-Based Recording Configuration**
- Conditionally rendered (only when "Recording during selected times only" is selected)
- Components needed:
  - Day of week checkboxes (Sunday - Saturday)
  - Start time picker (HH:MM format)
  - End time picker (HH:MM format)
- Logic: If both start and end time are "00:00", treat as 24-hour recording

**State Management:**
```typescript
interface RecordingConfig {
  location: 'sd_card' | 'local_storage';
  mode: 'motion' | 'constant' | 'scheduled';
  schedule?: {
    days: {
      sunday: boolean;
      monday: boolean;
      tuesday: boolean;
      wednesday: boolean;
      thursday: boolean;
      friday: boolean;
      saturday: boolean;
    };
    startTime: string; // "HH:MM"
    endTime: string;   // "HH:MM"
  };
}
```

---

### Feature 4: LED Configuration Page (NEW)

#### 4.1 Menu Entry
**Location:** `www/src/layout/mobile/sidebar/index.tsx` (and PC equivalent)

**Requirements:**
- Add new "LED Configuration" menu item
- Create appropriate icon (lightbulb or LED symbol)
- Add route configuration for `/led-config` path

#### 4.2 LED Configuration View
**Location:** `www/src/views/led-config/index.tsx` (NEW FILE)

**Requirements:**
Create a view with toggle switches for each LED:

| LED Type | Control |
|----------|---------|
| White Light LEDs | Toggle (On/Off) |
| Blue LED | Toggle (On/Off) |
| Red LED | Toggle (On/Off) |
| Green LED | Toggle (On/Off) |

**Component Structure:**
- Use Ant Design Switch components
- Each LED should have a label and independent toggle
- Consider adding LED status indicators (visual feedback)

**State Management:**
```typescript
interface LEDConfig {
  whiteLED: boolean;
  blueLED: boolean;
  redLED: boolean;
  greenLED: boolean;
}
```

---

### Feature 5: Updates Menu Section (NEW)

#### 5.1 Menu Entry
**Location:** `www/src/layout/mobile/sidebar/index.tsx` (and PC equivalent)

**Requirements:**
- Add new "Updates" menu item
- Add route configuration for `/updates` path

#### 5.2 Updates View
**Location:** `www/src/views/updates/index.tsx` (NEW FILE)

**Requirements:**

**A. Camera OS Updates Card**
- Combine existing Update and Beta channel functionality
- Card title: "Camera OS Updates"
- Components:
  - Current version display
  - Available update info (if any)
  - Channel selector (replace bottom scroller with clear dropdown/radio):
    - Stable
    - Beta
  - "Check for Updates" button
  - "Install Update" button (when update available)

**B. Camera Model Updates Card**
- Mirror the Camera OS Updates card layout
- Card title: "Camera Model Updates"
- Same features as Camera OS Updates card

**C. Update Check Frequency Selector**
- Dropdown or radio group with options:
  - "Every 30 minutes"
  - "Daily"
  - "Weekly" (shows additional day-of-week selector when selected)
  - "Manual only"

**State Management:**
```typescript
interface UpdateConfig {
  osChannel: 'stable' | 'beta';
  modelChannel: 'stable' | 'beta';
  checkFrequency: '30min' | 'daily' | 'weekly' | 'manual';
  weeklyDay?: 'sunday' | 'monday' | 'tuesday' | 'wednesday' | 'thursday' | 'friday' | 'saturday';
}
```

---

### Feature 6: About Menu Section (NEW)

#### 6.1 Menu Entry
**Location:** `www/src/layout/mobile/sidebar/index.tsx` (and PC equivalent)

**Requirements:**
- Add new "About" menu item
- Add route configuration for `/about` path

#### 6.2 About View
**Location:** `www/src/views/about/index.tsx` (NEW FILE)

**Requirements:**

**A. Links Section**
- "Open Source Licenses" link (opens license page/modal)
- "Data Collection Policy" link (opens policy page/modal)

**B. Footer Message**
- Friendly, reassuring message
- Suggested text options:
  - "Made with thought for you, the user."
  - "We care more about your security than the original developer."
- Style: Centered, subtle typography, warm tone

**Component Structure:**
```tsx
<AboutView>
  <LinksSection>
    <Link to="/licenses">Open Source Licenses</Link>
    <Link to="/privacy">Data Collection Policy</Link>
  </LinksSection>
  <FooterMessage>
    Made with thought for you, the user.
  </FooterMessage>
</AboutView>
```

---

### Feature 7: Storage Usage Display

#### 7.1 Storage Information Component
**Location:** `www/src/views/files/index.tsx` or new component

**Requirements:**
- Display storage information:
  - Total storage space (formatted: e.g., "32 GB")
  - Used storage space (formatted: e.g., "12.5 GB")
  - Visual progress bar showing usage percentage
- Storage source selector:
  - "Local Storage"
  - "SD Card"
- Information updates when source selection changes

**Component Structure:**
```tsx
<StorageInfo>
  <SourceSelector value={source} onChange={setSource} />
  <StorageBar used={usedSpace} total={totalSpace} />
  <StorageDetails>
    <span>Used: {usedSpace}</span>
    <span>Total: {totalSpace}</span>
  </StorageDetails>
</StorageInfo>
```

---

### Feature 8: WiFi Network List Behavior

#### 8.1 Network List Modification
**Location:** `www/src/views/network/index.tsx`

**Requirements:**
- Limit visible WiFi networks to 6 entries maximum
- If more than 6 networks exist:
  - Add vertical scrollbar to the list container
  - Set fixed height for list container (approximately 6 items height)
- Prevent list from expanding window height

**CSS Implementation:**
```css
.wifi-network-list {
  max-height: 360px; /* Approximately 6 items */
  overflow-y: auto;
}
```

---

## File Structure Changes

### New Files to Create
```
www/src/views/
├── recording/
│   └── index.tsx
├── led-config/
│   └── index.tsx
├── updates/
│   └── index.tsx
└── about/
    └── index.tsx
```

### Files to Modify
```
www/src/layout/mobile/header/index.tsx    # Hostname display
www/src/layout/mobile/sidebar/index.tsx   # New menu items
www/src/layout/pc/index.tsx               # PC layout updates
www/src/views/network/index.tsx           # Hostname section, WiFi list
www/src/views/files/index.tsx             # Upload readability, storage display
www/src/App.tsx                           # New routes
```

---

## Routing Configuration

Add the following routes to `App.tsx`:

```typescript
const routes = [
  // ... existing routes
  { path: '/recording', component: RecordingView },
  { path: '/led-config', component: LEDConfigView },
  { path: '/updates', component: UpdatesView },
  { path: '/about', component: AboutView },
];
```

---

## UI Component Guidelines

### Consistent Styling
- Use existing Tailwind CSS classes where possible
- Follow Ant Design component patterns
- Maintain responsive design (mobile/PC layouts)

### Accessibility
- All interactive elements must be keyboard accessible
- Use appropriate ARIA labels
- Maintain sufficient color contrast

### State Management
- Use existing state management patterns in the codebase
- API calls should use existing service layer

---

## Testing Considerations

- Verify all new routes are accessible
- Test responsive behavior on mobile and desktop
- Validate form inputs (hostname, time pickers)
- Test conditional rendering (scheduled recording options)
- Verify storage display updates correctly when source changes

---

## Questions for Clarification

1. **Recording Feature:** Should the recording configuration persist immediately on change, or require a "Save" button?

2. **LED Configuration:** Are there any LED combinations that should be mutually exclusive?

3. **Updates:** Should the update check frequency setting trigger an immediate check when changed?

4. **Storage Display:** Should the storage information auto-refresh, or only update on manual refresh?

5. **About Page:** Are there specific URLs for the Open Source Licenses and Data Collection Policy pages, or should these be modal dialogs?

---

## Acceptance Criteria Summary

| Feature | Criteria |
|---------|----------|
| Hostname Display | Shows "Camera Name: <hostname>" in top bar, edit icon visible |
| Upload Window | Improved readability, prominent Upload button |
| Recording Menu | New menu item, location/mode selectors, time-based config |
| LED Config | New menu item, 4 independent LED toggles |
| Updates Menu | Combined OS card, Model card, frequency selector |
| About Menu | Links to licenses/policy, friendly footer message |
| Storage Display | Shows used/total, updates based on source selection |
| WiFi List | Max 6 visible items, scrollable if more |

---

*This specification is ready for implementation review. Please confirm understanding before development begins.*
