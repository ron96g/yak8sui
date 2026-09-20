# Make ownership table rows the preview action

Use each ownership table row as the single action for selecting its resource's YAML preview. Remove the redundant Actions column and Preview button, and make interactivity visible through pointer, hover, and keyboard-focus treatments. Keep disclosure controls independent so expanding a branch does not change the preview. This supersedes the visible-actions requirement in ADRs 0033 and 0040 for the table presentation.

Give the Ownership tab a wider dialog and allocate more of its desktop split to the hierarchy than to the YAML preview. Preserve horizontal scrolling on narrow screens and retain keyboard activation with Enter or Space.
