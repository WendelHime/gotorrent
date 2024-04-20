Feature: download torrent files from the web
    In order to download torrent files from the web
    As a user
    I want to be able to download files through the bittorrent protocol

    Scenario: download a torrent file
        Given I have a torrent file "sample.torrent"
        When I download the file
        Then The output hash should match "1577533193d6eaf67fa97e1d5bc9d1dfbe4f82e3" 
