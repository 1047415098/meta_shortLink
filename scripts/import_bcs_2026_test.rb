# frozen_string_literal: true

require "minitest/autorun"
require "socket"
require "webrick"
require_relative "import_bcs_2026"

class ImportBcs2026Test < Minitest::Test
  ARCHIVE_HTML = <<~HTML
    <html><body>
      <div class="post-title">
        <a href="/issues/issue-466/">Issue #466</a>
        <div class="post-date">September 17, 2026</div>
      </div>
      <div class="post-body">
        <div class="cover-image"><img src="https://example.com/issue-466.jpg"></div>
        <div class="issue-story-title"><a href="https://beneath-ceaseless-skies.com/stories/first-story/">First Story</a></div>
        <div class="issue-author-name">Author One</div>
        <div class="issue-story-excerpt long"><p>Long first excerpt.</p></div>
        <div class="issue-story-excerpt short">Short first excerpt.</div>
        <div class="issue-story-title"><a href="https://beneath-ceaseless-skies.com/stories/second-story/">Second Story</a></div>
        <div class="issue-author-name">Author Two</div>
        <div class="issue-story-excerpt">Second excerpt.</div>
        <div class="issue-podcast-subheader">Audio Fiction Podcast:</div>
        <div class="issue-story-title"><a href="https://beneath-ceaseless-skies.com/audio/second-story/">Second Story</a></div>
        <div class="powerpress_player"><audio><source type="audio/mpeg" src="https://example.com/second-story.mp3?_=1"></audio></div>
        <p>Podcast: <a href="https://example.com/second-story.mp3">Download</a> (Duration: 32:05 — 22.03MB)</p>
        <div class="issue-podcast-subheader">Audio Fiction Podcast:</div>
        <div class="issue-story-title"><a href="https://beneath-ceaseless-skies.com/audio/missing-story/">Missing Story</a></div>
        <div class="powerpress_player"><audio><source type="audio/mpeg" src="https://example.com/missing.mp3"></audio></div>
        <p>Podcast: <a href="https://example.com/missing.mp3">Download</a> (Duration: 10:00 — 7.00MB)</p>
        <div class="issue-podcast-subheader">From the Archives:</div>
        <div class="issue-story-title"><a href="https://beneath-ceaseless-skies.com/stories/old-story/">Old Story</a></div>
      </div>
    </body></html>
  HTML

  STORY_HTML = <<~HTML
    <html><head>
      <meta property="og:description" content="Story description">
    </head><body>
      <div class="bcs-story-content">
        <p>First paragraph with <em>emphasis</em>.</p>
        <p>Second paragraph.<br>New line.</p>
      </div>
      <div class="author-bio"><p>This must not be imported.</p></div>
    </body></html>
  HTML

  def test_archive_parser_keeps_only_current_issue_stories
    stories = BcsImport.parse_archive(ARCHIVE_HTML)

    assert_equal %w[first-story second-story], stories.map { |story| story.fetch(:slug) }
    assert_equal ["Short first excerpt.", "Second excerpt."], stories.map { |story| story.fetch(:excerpt) }
    assert_equal ["2026-09-17"], stories.map { |story| story.fetch(:published_at) }.uniq
    assert_equal ["https://example.com/issue-466.jpg"], stories.map { |story| story.fetch(:cover_url) }.uniq
  end

  def test_story_parser_converts_only_story_body_to_markdown
    markdown = BcsImport.parse_story_body(STORY_HTML)

    assert_equal "First paragraph with *emphasis*.\n\nSecond paragraph.\nNew line.", markdown
    refute_includes markdown, "This must not be imported"
  end

  def test_podcast_parser_matches_formal_story_slug_and_reports_unmatched_title
    unmatched = []
    podcasts = BcsImport.parse_podcasts(ARCHIVE_HTML, unmatched: unmatched)

    assert_equal [{
      slug: "second-story",
      audio_url: "https://example.com/second-story.mp3",
      duration: "32:05",
      declared_size: "22.03MB"
    }], podcasts
    assert_equal ["Missing Story"], unmatched
  end

  def test_audio_import_streams_cleans_up_and_continues_after_failures
    port_probe = TCPServer.new("127.0.0.1", 0)
    port = port_probe.addr[1]
    port_probe.close
    server = WEBrick::HTTPServer.new(BindAddress: "127.0.0.1", Port: port, Logger: WEBrick::Log.new(File::NULL), AccessLog: [])
    server.mount_proc("/valid.mp3") { |_request, response| response.body = "ID3valid-audio" }
    server.mount_proc("/oversized.mp3") { |_request, response| response.body = "ID3" + ("x" * 30) }
    server.mount_proc("/truncated.mp3") do |_request, response|
      response.chunked = true
      response.body = proc do |output|
        output.write("ID3partial")
        raise IOError, "connection cut"
      end
    end
    server.mount_proc("/later.mp3") { |_request, response| response.body = "\xff\xfblater-audio".b }
    thread = Thread.new { server.start }
    Thread.pass until server.status == :Running

    client = Class.new do
      attr_reader :uploaded_paths, :updates
      def initialize
        @uploaded_paths = []
        @updates = []
      end
      def upload_audio(file)
        @uploaded_paths << file.path
        { "path" => "/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3", "size_bytes" => file.size }
      end
      def update_story(id, payload)
        @updates << [id, payload]
      end
    end.new
    stories = %w[valid oversized truncated later].each_with_index.each_with_object({}) do |(slug, index), result|
      result[slug] = { "id" => index + 1, "title" => slug, "slug" => slug, "category" => "Fantasy", "excerpt" => "Excerpt", "body_markdown" => "Body", "cover_path" => "", "audio_path" => "", "audio_duration" => "", "audio_size_bytes" => 0, "published_at" => "2026-09-22", "enabled" => true, "featured" => false }
    end
    podcasts = %w[valid oversized truncated later].map do |slug|
      { slug: slug, audio_url: "http://127.0.0.1:#{port}/#{slug}.mp3", duration: "01:05", declared_size: "1MB" }
    end

    result = BcsImport.import_podcasts(podcasts: podcasts, stories: stories, client: client, apply: true, refresh_audio: false, max_audio_bytes: 16)

    assert_equal %w[valid later], result.fetch(:attached)
    assert_equal %w[oversized truncated], result.fetch(:failed).map { |item| item.fetch(:slug) }
    assert_equal [1, 4], client.updates.map(&:first)
    assert client.uploaded_paths.all? { |path| !File.exist?(path) }, "temporary MP3 files were not removed"
  ensure
    server&.shutdown
    thread&.join
  end

  def test_merge_details_skips_existing_slugs_without_overwriting
    stories = BcsImport.parse_archive(ARCHIVE_HTML)
    pending = BcsImport.pending_stories(stories, ["first-story"])

    assert_equal ["second-story"], pending.map { |story| story.fetch(:slug) }
  end

  def test_admin_client_logs_in_and_reads_existing_slugs_on_ruby_26
    # 真实 HTTP 测试用于锁定 Ruby 2.6 的位置参数与关键字参数兼容行为。
    port_probe = TCPServer.new("127.0.0.1", 0)
    port = port_probe.addr[1]
    port_probe.close
    server = WEBrick::HTTPServer.new(BindAddress: "127.0.0.1", Port: port, Logger: WEBrick::Log.new(File::NULL), AccessLog: [])
    server.mount_proc("/api/v1/auth/login") do |request, response|
      credentials = JSON.parse(request.body)
      response.status = credentials == { "username" => "admin", "password" => "secret" } ? 200 : 401
      response["Set-Cookie"] = "session=test-cookie; Path=/"
      response["Content-Type"] = "application/json"
      response.body = JSON.generate(username: "admin")
    end
    server.mount_proc("/api/v1/audio-novels") do |request, response|
      response.status = request["Cookie"] == "session=test-cookie" ? 200 : 401
      response["Content-Type"] = "application/json"
      response.body = JSON.generate(items: [{ slug: "existing-story" }])
    end
    thread = Thread.new { server.start }
    # 等待服务器进入运行态，避免异常路径先 shutdown、后 start 导致测试线程悬挂。
    Thread.pass until server.status == :Running

    client = BcsImport::AdminClient.new(
      base_url: "http://127.0.0.1:#{port}",
      username: "admin",
      password: "secret"
    )
    client.login

    assert_equal ["existing-story"], client.existing_slugs
  ensure
    server&.shutdown
    thread&.join
  end
end
