wtff is a frontend for some ffmpeg operations. ffmpeg is really neat and
powerfull, but some incatations of the CLI are obscure enough that I can never
be quite sure if it will do what I want to do, or will spawn a demon from the
darkest depths of hell. Who knows? It's possible.

It's not intended to be a flexible tool to solve every problem; just some common
things for common usage. Basically: stuff I wanted to do with ffmpeg at some
point, with a bit of work to make it *slightly* generic.

Install with:

    go install zgo.at/wtff

Commands ("wtff" or "wtff help" for more info):

    info         Show file information.
    meta         Edit metadata in $EDITOR
    mb           Load metadata from MusicBrainz
    cat          Join one or more files.
    cut          Cut a part from a file.
    sub add      Add subtitle.
    sub rm       Remove subtitle.
    sub save     Save subtitle to file.
    sub print    Print subtitle to stdout.
    audio add    Add audio track.
    audio rm     Remove audio track
    audio save   Save audio track to file.
