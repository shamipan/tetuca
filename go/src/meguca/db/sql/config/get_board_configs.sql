select readOnly, textOnly, forcedAnon, disableRobots, flags, NSFW, posterIDs,
		id,	defaultCSS, title, notice, rules, eightball
	from boards
	where id = $1
