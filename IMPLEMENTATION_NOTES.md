# Implementation Notes

## New Files Created

1. **media.go**
   - Core data structures and database operations for media suggestions
   - Functions for adding, retrieving, and updating suggestions
   - Utility functions for extracting URLs from messages and formatting media lists

2. **scheduler.go**
   - Responsible for scheduling and sending periodic notifications
   - Monday 15:00 Berlin: Reminds about media collection
   - Wednesday 12:00 Berlin: Presents media list for selection
   - Sunday 17:00 Berlin: Reminds about upcoming discussion

3. **media_handler.go**
   - Functions for processing media-related Telegram messages
   - Detects suggestions and adds them to the database
   - Handles media selection by the owner
   - Provides commands to view and manage current media list

## Changes to Existing Files

1. **main.go**
   - Added initialization of media suggestions table
   - Started the scheduler for media tasks

2. **telegram.go**
   - Need to integrate the media suggestion detection
   - Need to add handlers for:
     - `/media` and `/list` commands to show suggestions
     - `/del [number]` command to delete suggestions
     - Media selection by the owner

3. **Dockerfile**
   - Added documentation for the CLASS_GROUP_ID environment variable

4. **CLAUDE.md**
   - Updated with information about the media management features
   - Added the new CLASS_GROUP_ID environment variable

## Environment Variables

- `OWNER_ID`: Used for identifying the bot owner who can select media
- `CLASS_GROUP_ID`: Used to identify which group chat to monitor for suggestions

## Integration Instructions

To fully integrate this functionality, you'll need to manually apply the changes described in the `telegram_integration.txt` file to your `telegram.go` file. The integration points should be:

1. After adding user activity (around line 73)
2. Before checking if the message contains the bot's name (around line 434)

These changes will enable the bot to:
- Detect and store suggestions
- Respond to the /media and /list commands
- Allow users to delete their own suggestions with /del
- Allow the owner to delete any suggestion with /del
- Process media selection by the owner
- Send scheduled announcements

The scheduler will handle the weekly notifications and management of the media suggestions.